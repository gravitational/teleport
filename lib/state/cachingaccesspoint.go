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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	accessPointRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "access_point_requests",
			Help: "Number of access point requests",
		},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(accessPointRequests)
}

// CachingAuthClient implements auth.AccessPoint interface and remembers
// the previously returned upstream value for each API call.
//
// This which can be used if the upstream AccessPoint goes offline
type CachingAuthClient struct {
	Config
	*log.Entry
	// ap points to the access point we're caching access to:
	ap auth.AccessPoint

	// lastErrorTime is a timestamp of the last error when talking to the AP
	lastErrorTime time.Time

	identity services.Identity
	access   services.Access
	trust    services.Trust
	presence services.Presence
	config   services.ClusterConfiguration
}

// Config is CachingAuthClient config
type Config struct {
	// CacheTTL sets maximum TTL the cache keeps the value
	CacheTTL time.Duration
	// NeverExpires if set, never expires cache values
	NeverExpires bool
	// AccessPoint is access point for this
	AccessPoint auth.AccessPoint
	// Backend is cache backend
	Backend backend.Backend
	// Clock can be set to control time
	Clock clockwork.Clock
	// SkipPreload turns off preloading on start
	SkipPreload bool
}

// CheckAndSetDefaults checks parameters and sets default values
func (c *Config) CheckAndSetDefaults() error {
	if !c.NeverExpires && c.CacheTTL == 0 {
		c.CacheTTL = defaults.CacheTTL
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint parameter")
	}
	if c.Backend == nil {
		return trace.BadParameter("missing Backend parameter")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewCachingAuthClient creates a new instance of CachingAuthClient using a
// live connection to the auth server (ap)
func NewCachingAuthClient(config Config) (*CachingAuthClient, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cs := &CachingAuthClient{
		Config:   config,
		ap:       config.AccessPoint,
		identity: local.NewIdentityService(config.Backend),
		trust:    local.NewCAService(config.Backend),
		access:   local.NewAccessService(config.Backend),
		presence: local.NewPresenceService(config.Backend),
		config:   local.NewClusterConfigurationService(config.Backend),
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentCachingClient,
		}),
	}
	if !cs.SkipPreload {
		err := cs.fetchAll()
		if err != nil {
			// we almost always get some "access denied" errors here because
			// not all cacheable resources are available (for example nodes do
			// not have access to tunnels)
			cs.Debugf("auth cache: %v", err)
		}
	}
	return cs, nil
}

func (cs *CachingAuthClient) fetchAll() error {
	var errors []error
	_, err := cs.GetDomainName()
	errors = append(errors, err)
	_, err = cs.GetClusterConfig()
	errors = append(errors, err)
	_, err = cs.GetRoles()
	errors = append(errors, err)
	namespaces, err := cs.GetNamespaces()
	errors = append(errors, err)
	if err == nil {
		for _, ns := range namespaces {
			_, err = cs.GetNodes(ns.Metadata.Name)
			errors = append(errors, err)
		}
	}
	_, err = cs.GetProxies()
	errors = append(errors, err)
	_, err = cs.GetReverseTunnels()
	errors = append(errors, err)
	_, err = cs.GetCertAuthorities(services.UserCA, false)
	errors = append(errors, err)
	_, err = cs.GetCertAuthorities(services.HostCA, false)
	errors = append(errors, err)
	_, err = cs.GetUsers()
	errors = append(errors, err)
	conns, err := cs.ap.GetAllTunnelConnections()
	if err != nil {
		errors = append(errors, err)
	}
	clusters := map[string]bool{}
	for _, conn := range conns {
		clusterName := conn.GetClusterName()
		if _, ok := clusters[clusterName]; ok {
			continue
		}
		clusters[clusterName] = true
		_, err = cs.GetTunnelConnections(clusterName)
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// GetDomainName is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetDomainName() (clusterName string, err error) {
	err = cs.try(func() error {
		clusterName, err = cs.ap.GetDomainName()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetLocalClusterName()
		}
		return clusterName, err
	}
	if err = cs.presence.UpsertLocalClusterName(clusterName); err != nil {
		return "", trace.Wrap(err)
	}
	return clusterName, err
}

func (cs *CachingAuthClient) GetClusterConfig() (clusterConfig services.ClusterConfig, err error) {
	err = cs.try(func() error {
		clusterConfig, err = cs.ap.GetClusterConfig()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.config.GetClusterConfig()
		}
		return nil, trace.Wrap(err)
	}
	if err = cs.config.SetClusterConfig(clusterConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return clusterConfig, nil
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRoles() (roles []services.Role, err error) {
	err = cs.try(func() error {
		roles, err = cs.ap.GetRoles()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.access.GetRoles()
		}
		return roles, err
	}
	if err := cs.access.DeleteAllRoles(); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, role := range roles {
		if err := cs.access.UpsertRole(role, backend.Forever); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return roles, err
}

func (cs *CachingAuthClient) setTTL(r services.Resource) {
	if cs.NeverExpires {
		return
	}
	// honor expiry set by user
	if !r.Expiry().IsZero() {
		return
	}
	// set TTL as a global setting
	r.SetTTL(cs.Clock, cs.CacheTTL)
}

// GetRole is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRole(name string) (role services.Role, err error) {
	err = cs.try(func() error {
		role, err = cs.ap.GetRole(name)
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.access.GetRole(name)
		}
		return role, err
	}
	if err := cs.access.DeleteRole(name); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	cs.setTTL(role)
	if err := cs.access.UpsertRole(role, backend.Forever); err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// GetNamespace returns namespace
func (cs *CachingAuthClient) GetNamespace(name string) (namespace *services.Namespace, err error) {
	err = cs.try(func() error {
		namespace, err = cs.ap.GetNamespace(name)
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetNamespace(name)
		}
		return namespace, err
	}
	if err := cs.presence.DeleteNamespace(name); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	if err := cs.presence.UpsertNamespace(*namespace); err != nil {
		return nil, trace.Wrap(err)
	}
	return namespace, err
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNamespaces() (namespaces []services.Namespace, err error) {
	err = cs.try(func() error {
		namespaces, err = cs.ap.GetNamespaces()
		return err
	})

	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetNamespaces()
		}
		return namespaces, err
	}
	if err := cs.presence.DeleteAllNamespaces(); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, ns := range namespaces {
		if err := cs.presence.UpsertNamespace(ns); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return namespaces, err
}

// GetNodes is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNodes(namespace string) (nodes []services.Server, err error) {
	err = cs.try(func() error {
		nodes, err = cs.ap.GetNodes(namespace)
		return err

	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetNodes(namespace)
		}
		return nodes, err
	}
	if err := cs.presence.DeleteAllNodes(namespace); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, node := range nodes {
		cs.setTTL(node)
		if err := cs.presence.UpsertNode(node); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return nodes, err
}

func (cs *CachingAuthClient) GetReverseTunnels() (tunnels []services.ReverseTunnel, err error) {
	err = cs.try(func() error {
		tunnels, err = cs.ap.GetReverseTunnels()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetReverseTunnels()
		}
		return tunnels, err
	}
	if err := cs.presence.DeleteAllReverseTunnels(); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, tunnel := range tunnels {
		cs.setTTL(tunnel)
		if err := cs.presence.UpsertReverseTunnel(tunnel); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return tunnels, err
}

// GetProxies is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetProxies() (proxies []services.Server, err error) {
	err = cs.try(func() error {
		proxies, err = cs.ap.GetProxies()
		return err
	})

	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetProxies()
		}
		return proxies, err
	}
	if err := cs.presence.DeleteAllProxies(); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, proxy := range proxies {
		cs.setTTL(proxy)
		if err := cs.presence.UpsertProxy(proxy); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return proxies, err
}

// GetCertAuthority is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthority(id services.CertAuthID, loadKeys bool) (ca services.CertAuthority, err error) {
	err = cs.try(func() error {
		ca, err = cs.ap.GetCertAuthority(id, loadKeys)
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.trust.GetCertAuthority(id, loadKeys)
		}
		return ca, err
	}
	if err := cs.trust.DeleteCertAuthority(id); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	cs.setTTL(ca)
	if err := cs.trust.UpsertCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, err
}

// GetCertAuthorities is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthorities(ct services.CertAuthType, loadKeys bool) (cas []services.CertAuthority, err error) {
	err = cs.try(func() error {
		cas, err = cs.ap.GetCertAuthorities(ct, loadKeys)
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.trust.GetCertAuthorities(ct, loadKeys)
		}
		return cas, err
	}
	if err := cs.trust.DeleteAllCertAuthorities(ct); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, ca := range cas {
		cs.setTTL(ca)
		if err := cs.trust.UpsertCertAuthority(ca); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return cas, err
}

// GetUsers is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetUsers() (users []services.User, err error) {
	err = cs.try(func() error {
		users, err = cs.ap.GetUsers()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.identity.GetUsers()
		}
		return users, err
	}
	if err := cs.identity.DeleteAllUsers(); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, user := range users {
		cs.setTTL(user)
		if err := cs.identity.UpsertUser(user); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return users, err
}

// GetTunnelConnections is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetTunnelConnections(clusterName string) (conns []services.TunnelConnection, err error) {
	err = cs.try(func() error {
		conns, err = cs.ap.GetTunnelConnections(clusterName)
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetTunnelConnections(clusterName)
		}
		return conns, err
	}
	if err := cs.presence.DeleteTunnelConnections(clusterName); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, conn := range conns {
		cs.setTTL(conn)
		if err := cs.presence.UpsertTunnelConnection(conn); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return conns, err
}

// GetAllTunnelConnections is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetAllTunnelConnections() (conns []services.TunnelConnection, err error) {
	err = cs.try(func() error {
		conns, err = cs.ap.GetAllTunnelConnections()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			return cs.presence.GetAllTunnelConnections()
		}
		return conns, err
	}
	if err := cs.presence.DeleteAllTunnelConnections(); err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	for _, conn := range conns {
		cs.setTTL(conn)
		if err := cs.presence.UpsertTunnelConnection(conn); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return conns, err
}

// UpsertNode is part of auth.AccessPoint implementation
func (cs *CachingAuthClient) UpsertNode(s services.Server) error {
	cs.setTTL(s)
	return cs.ap.UpsertNode(s)
}

// UpsertProxy is part of auth.AccessPoint implementation
func (cs *CachingAuthClient) UpsertProxy(s services.Server) error {
	cs.setTTL(s)
	return cs.ap.UpsertProxy(s)
}

// UpsertTunnelConnection is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) UpsertTunnelConnection(conn services.TunnelConnection) error {
	cs.setTTL(conn)
	return cs.ap.UpsertTunnelConnection(conn)
}

// DeleteTunnelConnection is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) DeleteTunnelConnection(clusterName, connName string) error {
	return cs.ap.DeleteTunnelConnection(clusterName, connName)
}

// try calls a given function f and checks for errors. If f() fails, the current
// time is recorded. Future calls to f will be ingored until sufficient time passes
// since th last error
func (cs *CachingAuthClient) try(f func() error) error {
	tooSoon := cs.lastErrorTime.Add(defaults.NetworkRetryDuration).After(time.Now())
	if tooSoon {
		cs.Warnf("backoff: using cached value due to recent errors")
		return trace.ConnectionProblem(fmt.Errorf("backoff"), "backing off due to recent errors")
	}
	accessPointRequests.Inc()
	err := trace.ConvertSystemError(f())
	if trace.IsConnectionProblem(err) {
		cs.lastErrorTime = time.Now()
		cs.Warningf("connection problem: failed connect to the auth servers, using local cache")
	}
	return err
}
