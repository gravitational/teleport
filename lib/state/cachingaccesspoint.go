/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package state

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

const (
	backoffDuration = time.Second * 10
)

// CachingAuthClient implements auth.AccessPoint interface and remembers
// the previously returned upstream value for each API call.
//
// This which can be used if the upstream AccessPoint goes offline
type CachingAuthClient struct {
	// ap points to the access ponit we're caching access to:
	ap auth.AccessPoint

	// lastErrorTime is a timestamp of the last error when talking to the AP
	lastErrorTime time.Time

	identity services.Identity
	access   services.Access
	trust    services.Trust
	presence services.Presence
}

// NewCachingAuthClient creates a new instance of CachingAuthClient using a
// live connection to the auth server (ap)
func NewCachingAuthClient(ap auth.AccessPoint, b backend.Backend) (*CachingAuthClient, error) {
	cs := &CachingAuthClient{
		ap:       ap,
		identity: local.NewIdentityService(b),
		trust:    local.NewCAService(b),
		access:   local.NewAccessService(b),
		presence: local.NewPresenceService(b),
	}
	go func() {
		err := cs.fetchAll()
		if err != nil {
			log.Warningf("failed to fetch cached results %v", err)
		}
	}()
	return cs, nil
}

// CacheProblem indicates failure to update local cache
func CacheProblem(err error) error {
	return trace.WrapWithMessage(&trace.BadParameterError{
		Message: err.Error(),
	}, "failed to update local cache")
}

func (cs *CachingAuthClient) fetchAll() error {
	var errors []error
	_, err := cs.GetDomainName()
	errors = append(errors, err)
	_, err = cs.GetRoles()
	errors = append(errors, err)
	ns, err := cs.GetNamespaces()
	errors = append(errors, err)
	if err == nil {
		for _, n := range ns {
			cs.GetNodes(n.Metadata.Name)
		}
	}
	_, err = cs.GetProxies()
	errors = append(errors, err)
	_, err = cs.GetCertAuthorities(services.UserCA, false)
	errors = append(errors, err)
	_, err = cs.GetCertAuthorities(services.HostCA, false)
	errors = append(errors, err)
	_, err = cs.GetUsers()
	errors = append(errors, err)
	return trace.NewAggregate(errors...)
}

// GetDomainName is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetDomainName() (clusterName string, err error) {
	err = cs.try(func() error {
		clusterName, err = cs.ap.GetDomainName()
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return clusterName, err
	}
	if err == nil {
		cs.presence.UpsertLocalClusterName(clusterName)
	}
	return cs.presence.GetLocalClusterName()
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRoles() (roles []services.Role, err error) {
	err = cs.try(func() error {
		roles, err = cs.ap.GetRoles()
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return roles, err
	}
	// update cache if we got results
	if err == nil {
		if err := cs.access.DeleteAllRoles(); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		for _, role := range roles {
			if err := cs.access.UpsertRole(role); err != nil {
				return nil, CacheProblem(err)
			}
		}
	}
	return cs.access.GetRoles()
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRole(name string) (role services.Role, err error) {
	err = cs.try(func() error {
		role, err = cs.ap.GetRole(name)
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return role, err
	}
	if err == nil {
		if err := cs.access.DeleteRole(name); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		if err := cs.access.UpsertRole(role); err != nil {
			return nil, CacheProblem(err)
		}
	}
	return cs.access.GetRole(name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNamespaces() (namespaces []services.Namespace, err error) {
	err = cs.try(func() error {
		namespaces, err = cs.ap.GetNamespaces()
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return namespaces, err
	}
	if err == nil {
		if err := cs.presence.DeleteAllNamespaces(); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		for _, ns := range namespaces {
			if err := cs.presence.UpsertNamespace(ns); err != nil {
				return nil, CacheProblem(err)
			}
		}
	}
	return cs.presence.GetNamespaces()
}

// GetNodes is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNodes(namespace string) (nodes []services.Server, err error) {
	err = cs.try(func() error {
		nodes, err = cs.ap.GetNodes(namespace)
		return err

	})
	if !trace.IsConnectionProblem(err) {
		return nodes, err
	}
	if err == nil {
		if err := cs.presence.DeleteAllNodes(namespace); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		for _, node := range nodes {
			if err := cs.presence.UpsertNode(node, backend.Forever); err != nil {
				return nil, CacheProblem(err)
			}
		}
	}
	return cs.presence.GetNodes(namespace)
}

// GetProxies is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetProxies() (proxies []services.Server, err error) {
	err = cs.try(func() error {
		proxies, err = cs.ap.GetProxies()
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return proxies, err
	}
	if err == nil {
		if err := cs.presence.DeleteAllProxies(); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		for _, proxy := range proxies {
			if err := cs.presence.UpsertProxy(proxy, backend.Forever); err != nil {
				return nil, CacheProblem(err)
			}
		}
	}
	return cs.presence.GetProxies()
}

// GetCertAuthorities is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthorities(ct services.CertAuthType, loadKeys bool) (cas []services.CertAuthority, err error) {
	err = cs.try(func() error {
		cas, err = cs.ap.GetCertAuthorities(ct, loadKeys)
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return cas, err
	}
	if err == nil {
		if err := cs.trust.DeleteAllCertAuthorities(ct); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		for _, ca := range cas {
			if err := cs.trust.UpsertCertAuthority(ca, backend.Forever); err != nil {
				return nil, CacheProblem(err)
			}
		}
	}
	return cs.trust.GetCertAuthorities(ct, loadKeys)
}

// GetUsers is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetUsers() (users []services.User, err error) {
	err = cs.try(func() error {
		users, err = cs.ap.GetUsers()
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return users, err
	}
	if err == nil {
		if err := cs.identity.DeleteAllUsers(); err != nil {
			if !trace.IsNotFound(err) {
				return nil, CacheProblem(err)
			}
		}
		for _, user := range users {
			if err := cs.identity.UpsertUser(user); err != nil {
				return nil, CacheProblem(err)
			}
		}
	}
	return cs.identity.GetUsers()
}

// UpsertNode is part of auth.AccessPoint implementation
func (cs *CachingAuthClient) UpsertNode(s services.Server, ttl time.Duration) error {
	return cs.ap.UpsertNode(s, ttl)
}

// UpsertProxy is part of auth.AccessPoint implementation
func (cs *CachingAuthClient) UpsertProxy(s services.Server, ttl time.Duration) error {
	return cs.ap.UpsertProxy(s, ttl)
}

// try calls a given function f and checks for errors. If f() fails, the current
// time is recorded. Future calls to f will be ingored until sufficient time passes
// since th last error
func (cs *CachingAuthClient) try(f func() error) error {
	tooSoon := cs.lastErrorTime.Add(backoffDuration).After(time.Now())
	if tooSoon {
		log.Warnf("Not calling auth access point due to recent errors. Using cached value instead")
		return trace.ConnectionProblem(fmt.Errorf("backoff"), "backing off due to recent errors")
	}
	err := trace.ConvertSystemError(f())
	log.Errorf("probs: %v", err)
	if trace.IsConnectionProblem(err) {
		cs.lastErrorTime = time.Now()
		log.Warningf("failed connecto to the auth servers, using local cache")
	}
	return err
}
