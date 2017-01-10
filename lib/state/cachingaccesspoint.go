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
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

const (
	backoffDuration = time.Second * 10
)

// CachingAuthClient implements auth.AccessPoint interface and remembers
// the previously returned upstream value for each API call.
//
// This which can be used if the upstream AccessPoint goes offline
type CachingAuthClient struct {
	sync.RWMutex

	// ap points to the access ponit we're caching access to:
	ap auth.AccessPoint

	// timestamp of the last error when talking to the AP
	lastErrorTime time.Time

	//
	// fields below are the cached values received from the AP:
	//

	domainName string
	namespaces []services.Namespace
	roles      []services.Role
	nodes      map[string][]services.Server
	proxies    []services.Server
	users      []services.User
	userCAs    []services.CertAuthority
	hostCAs    []services.CertAuthority
}

// NewCachingAuthClient creates a new instance of CachingAuthClient using a
// live connection to the auth server (ap)
func NewCachingAuthClient(ap auth.AccessPoint) (*CachingAuthClient, error) {
	// read everything from the auth access point:
	domainName, err := ap.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespaces, err := ap.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, err := ap.GetRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nodes := make(map[string][]services.Server, len(namespaces))
	for _, ns := range namespaces {
		nsNodes, err := ap.GetNodes(ns.Metadata.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		nodes[ns.Metadata.Name] = nsNodes
	}
	proxies, err := ap.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	users, err := ap.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userCAs, err := ap.GetCertAuthorities(services.UserCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostCAs, err := ap.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cs := &CachingAuthClient{
		roles:      roles,
		ap:         ap,
		domainName: domainName,
		nodes:      nodes,
		proxies:    proxies,
		users:      users,
		userCAs:    userCAs,
		hostCAs:    hostCAs,
	}
	return cs, nil
}

// GetDomainName is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetDomainName() (string, error) {
	cs.try(func() error {
		dn, err := cs.ap.GetDomainName()
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.domainName = dn
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	return cs.domainName, nil
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRoles() ([]services.Role, error) {
	cs.try(func() error {
		roles, err := cs.ap.GetRoles()
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.roles = roles
		}
		return err
	})
	return cs.roles, nil
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRole(name string) (services.Role, error) {
	cs.try(func() error {
		roles, err := cs.ap.GetRoles()
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.roles = roles
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	for i := range cs.roles {
		if cs.roles[i].GetName() == name {
			return cs.roles[i], nil
		}
	}
	return nil, trace.NotFound("role %v is not found", name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNamespaces() ([]services.Namespace, error) {
	cs.try(func() error {
		namespaces, err := cs.ap.GetNamespaces()
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.namespaces = namespaces
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	return cs.namespaces, nil
}

// GetNodes is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNodes(namespace string) ([]services.Server, error) {
	cs.try(func() error {
		nodes, err := cs.ap.GetNodes(namespace)
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.nodes[namespace] = nodes
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	return cs.nodes[namespace], nil
}

// GetProxies is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetProxies() ([]services.Server, error) {
	cs.try(func() error {
		proxies, err := cs.ap.GetProxies()
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.proxies = proxies
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	return cs.proxies, nil
}

// GetCertAuthorities is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthorities(ct services.CertAuthType, loadKeys bool) ([]services.CertAuthority, error) {
	cs.try(func() error {
		retval, err := cs.ap.GetCertAuthorities(ct, loadKeys)
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			if ct == services.UserCA {
				cs.userCAs = retval
			} else {
				cs.hostCAs = retval
			}
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	if ct == services.UserCA {
		return cs.userCAs, nil
	}
	return cs.hostCAs, nil
}

// GetUsers is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetUsers() ([]services.User, error) {
	cs.try(func() error {
		users, err := cs.ap.GetUsers()
		if err == nil {
			cs.Lock()
			defer cs.Unlock()
			cs.users = users
		}
		return err
	})
	cs.RLock()
	defer cs.RUnlock()
	return cs.users, nil
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
func (cs *CachingAuthClient) try(f func() error) {
	tooSoon := cs.lastErrorTime.Add(backoffDuration).After(time.Now())
	if tooSoon {
		log.Warnf("Not calling auth access point due to recent errors. Using cached value instead")
		return
	}
	if err := f(); err != nil {
		cs.lastErrorTime = time.Now()
	}
}
