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
	return cs, nil
}

// CacheProblem indicates failure to update local cache
func CacheProblem(err error) error {
	return trace.WrapWithMessage(&trace.BadParameterError{
		Message: err.Error(),
	}, "failed to update local cache")
}

// GetDomainName is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetDomainName() (clusterName string, err error) {
	err = cs.try(func() error {
		clusterName, err = cs.ap.GetDomainName()
		if err == nil {
			cs.presence.UpsertLocalClusterName(clusterName)
		}
		return err
	})
	if !trace.IsConnectionProblem(err) {
		return clusterName, err
	}
	return cs.presence.GetLocalClusterName()
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRoles() (roles []services.Role, err error) {
	err = cs.try(func() error {
		roles, err = cs.ap.GetRoles()
		if err != nil {
			return err
		}
		if err := cs.access.DeleteAllRoles(); err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		for _, role := range roles {
			if err := cs.access.UpsertRole(role); err != nil {
				return CacheProblem(err)
			}
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return roles, err
	}
	return cs.access.GetRoles()
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRole(name string) (role services.Role, err error) {
	err = cs.try(func() error {
		role, err = cs.ap.GetRole(name)
		if err != nil {
			return err
		}
		err = cs.access.DeleteRole(name)
		if err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		if err := cs.access.UpsertRole(role); err != nil {
			return CacheProblem(err)
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return role, err
	}
	return cs.access.GetRole(name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNamespaces() (namespaces []services.Namespace, err error) {
	err = cs.try(func() error {
		namespaces, err = cs.ap.GetNamespaces()
		if err != nil {
			return err
		}
		if err := cs.presence.DeleteAllNamespaces(); err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		for _, ns := range namespaces {
			if err := cs.presence.UpsertNamespace(ns); err != nil {
				return CacheProblem(err)
			}
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return namespaces, err
	}
	return cs.presence.GetNamespaces()
}

// GetNodes is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNodes(namespace string) (nodes []services.Server, err error) {
	err = cs.try(func() error {
		nodes, err = cs.ap.GetNodes(namespace)
		if err != nil {
			return err
		}
		if err := cs.presence.DeleteAllNodes(namespace); err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		for _, node := range nodes {
			if err := cs.presence.UpsertNode(node, backend.Forever); err != nil {
				return CacheProblem(err)
			}
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return nodes, err
	}
	return cs.presence.GetNodes(namespace)
}

// GetProxies is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetProxies() (proxies []services.Server, err error) {
	err = cs.try(func() error {
		proxies, err = cs.ap.GetProxies()
		if err != nil {
			return err
		}
		if err := cs.presence.DeleteAllProxies(); err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		for _, proxy := range proxies {
			if err := cs.presence.UpsertProxy(proxy, backend.Forever); err != nil {
				return CacheProblem(err)
			}
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return proxies, err
	}
	return cs.presence.GetProxies()
}

// GetCertAuthorities is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthorities(ct services.CertAuthType, loadKeys bool) (cas []services.CertAuthority, err error) {
	err = cs.try(func() error {
		cas, err = cs.ap.GetCertAuthorities(ct, loadKeys)
		if err != nil {
			return err
		}
		if err := cs.trust.DeleteAllCertAuthorities(ct); err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		for _, ca := range cas {
			if err := cs.trust.UpsertCertAuthority(ca, backend.Forever); err != nil {
				return CacheProblem(err)
			}
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return cas, err
	}
	return cs.trust.GetCertAuthorities(ct, loadKeys)
}

// GetUsers is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetUsers() (users []services.User, err error) {
	err = cs.try(func() error {
		users, err = cs.ap.GetUsers()
		if err != nil {
			return err
		}
		if err := cs.identity.DeleteAllUsers(); err != nil {
			if !trace.IsNotFound(err) {
				return CacheProblem(err)
			}
		}
		for _, user := range users {
			if err := cs.identity.UpsertUser(user); err != nil {
				return CacheProblem(err)
			}
		}
		return nil
	})
	if !trace.IsConnectionProblem(err) {
		return users, err
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
	err := f()
	if trace.IsConnectionProblem(err) {
		log.Warningf("failed connecto to the auth servers, using local cache")
	}
	if err != nil {
		cs.lastErrorTime = time.Now()
	}
	return err
}
