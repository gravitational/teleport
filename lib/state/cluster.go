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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

const (
	notSupportedError = "not supported"
)

// ClusterSnapshot represents a snapshot of the CA state. This is something client
// nodes can take (and save) so they can perform authentication even when CA
// is temporarily offline
type ClusterSnapshot struct {
	// implements AccessPoint interface of a CA
	auth.AccessPoint

	// pointer to the original AP:
	ap auth.AccessPoint

	domainName string
	nodes      []services.Server
	proxies    []services.Server
	users      []services.User
	userCAs    []*services.CertAuthority
	hostCAs    []*services.CertAuthority
}

// MakeClusterSnapshot creates a new instance of ClusterSnapshot using a live connection
// to the auth server (CA)
func MakeClusterSnapshot(ap auth.AccessPoint) (*ClusterSnapshot, error) {
	// read everything from the auth access point:
	domainName, err := ap.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nodes, err := ap.GetNodes()
	if err != nil {
		return nil, trace.Wrap(err)
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
	cs := &ClusterSnapshot{
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
func (cs *ClusterSnapshot) GetDomainName() (string, error) {
	return cs.domainName, nil
}

func (cs *ClusterSnapshot) GetNodes() ([]services.Server, error) {
	return cs.nodes, nil
}

func (cs *ClusterSnapshot) GetProxies() ([]services.Server, error) {
	return cs.proxies, nil
}
func (cs *ClusterSnapshot) GetCertAuthorities(ct services.CertAuthType, loadKeys bool) ([]*services.CertAuthority, error) {
	if ct == services.UserCA {
		return cs.userCAs, nil
	}
	return cs.hostCAs, nil
}
func (cs *ClusterSnapshot) GetUsers() ([]services.User, error) {
	return cs.users, nil
}

// UpsertNode is part of auth.AccessPoint implementation. This method is not supported
// by a snapshot.
func (cs *ClusterSnapshot) UpsertNode(s services.Server, ttl time.Duration) error {
	if cs.ap == nil {
		return trace.Errorf(notSupportedError)
	}
	return cs.ap.UpsertNode(s, ttl)
}

// UpsertProxy is part of auth.AccessPoint implementation. This method is not supported
// by a snapshot.
func (cs *ClusterSnapshot) UpsertProxy(s services.Server, ttl time.Duration) error {
	if cs.ap == nil {
		return trace.Errorf(notSupportedError)
	}
	return cs.ap.UpsertProxy(s, ttl)
}
