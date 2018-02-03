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
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
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
	accessPointLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "access_point_latency_microseconds",
			Help: "Latency for access point operations",
			// Buckets in microsecond latencies
			Buckets: prometheus.ExponentialBuckets(5000, 1.5, 15),
		},
	)
	cacheLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "access_point_cache_latency_microseconds",
			Help: "Latency for access point cache operations",
			// Buckets in microsecond latencies
			Buckets: prometheus.ExponentialBuckets(5000, 1.5, 15),
		},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(accessPointRequests)
	prometheus.MustRegister(accessPointLatencies)
	prometheus.MustRegister(cacheLatencies)
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

	// recentCache keeps track of items recently fetched
	// from auth servers, including not found response codes,
	// to avoid hitting database too often
	recentCache *ttlmap.TTLMap

	// mutex is to check access to ttl map
	sync.RWMutex
}

// Config is CachingAuthClient config
type Config struct {
	// CacheMaxTTL sets maximum TTL the cache keeps the value
	// in case if there is no connection to auth servers
	CacheMaxTTL time.Duration
	// RecentCacheMinTTL sets TTL for items
	// that were recently retrieved from auth servers
	// if set to 0, not turned on, if set to 1 second,
	// it means that value accessed within last 1 second or NotFound error
	// will be returned instead of using auth server
	RecentCacheTTL time.Duration
	// NeverExpires if set, never expire cache values
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
	if !c.NeverExpires && c.CacheMaxTTL == 0 {
		c.CacheMaxTTL = defaults.CacheTTL
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
	cache, err := ttlmap.New(defaults.AccessPointCachedValues, ttlmap.Clock(config.Clock))
	if err != nil {
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
		recentCache: cache,
	}
	if !cs.SkipPreload {
		err := cs.fetchAll()
		if err != nil {
			// we almost always get some "access denied" errors here because
			// not all cacheable resources are available (for example nodes do
			// not have access to tunnels)
			cs.Debugf("Auth cache: %v.", err)
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
	cs.fetch(params{
		// key is a key of the cached value
		key: "clusterName",
		// fetch will be called if cache has expired
		fetch: func() error {
			clusterName, err = cs.ap.GetDomainName()
			return err
		},
		useCache: func() error {
			clusterName, err = cs.presence.GetLocalClusterName()
			return err
		},
		updateCache: func() ([]string, error) {
			return nil, cs.presence.UpsertLocalClusterName(clusterName)
		},
	})
	return
}

func (cs *CachingAuthClient) GetClusterConfig() (clusterConfig services.ClusterConfig, err error) {
	cs.fetch(params{
		key: "clusterConfig",
		fetch: func() error {
			clusterConfig, err = cs.ap.GetClusterConfig()
			return err
		},
		useCache: func() error {
			clusterConfig, err = cs.config.GetClusterConfig()
			return err
		},
		updateCache: func() ([]string, error) {
			return nil, cs.config.SetClusterConfig(clusterConfig)
		},
	})
	return
}

func roleKey(name string) string {
	return strings.Join([]string{"roles", name}, "/")
}

// GetRoles is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRoles() (roles []services.Role, err error) {
	cs.fetch(params{
		key: "roles",
		fetch: func() error {
			roles, err = cs.ap.GetRoles()
			return err
		},
		useCache: func() error {
			roles, err = cs.access.GetRoles()
			return err
		},
		updateCache: func() (keys []string, cerr error) {
			if err := cs.access.DeleteAllRoles(); err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			}
			for _, role := range roles {
				cs.setTTL(role)
				if err := cs.access.UpsertRole(role, backend.Forever); err != nil {
					return nil, trace.Wrap(err)
				}
				keys = append(keys, roleKey(role.GetName()))
			}
			return
		},
	})
	return
}

// GetRole is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetRole(name string) (role services.Role, err error) {
	cs.fetch(params{
		key: roleKey(name),
		fetch: func() error {
			role, err = cs.ap.GetRole(name)
			return err
		},
		useCache: func() error {
			role, err = cs.access.GetRole(name)
			return err
		},
		updateCache: func() (keys []string, cerr error) {
			if err := cs.access.DeleteRole(name); err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			}
			cs.setTTL(role)
			if err := cs.access.UpsertRole(role, backend.Forever); err != nil {
				return nil, trace.Wrap(err)
			}
			return
		},
	})
	return
}

func namespacesKey() string {
	return "namespaces"
}

func namespaceKey(key string) string {
	return strings.Join([]string{"namespaces", key}, "/")
}

// GetNamespace returns namespace
func (cs *CachingAuthClient) GetNamespace(name string) (namespace *services.Namespace, err error) {
	cs.fetch(params{
		key: namespaceKey(name),
		fetch: func() error {
			namespace, err = cs.ap.GetNamespace(name)
			return err
		},
		useCache: func() error {
			namespace, err = cs.presence.GetNamespace(name)
			return err
		},
		updateCache: func() (keys []string, cerr error) {
			if err := cs.presence.DeleteNamespace(name); err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			}
			if err := cs.presence.UpsertNamespace(*namespace); err != nil {
				return nil, trace.Wrap(err)
			}
			return
		},
	})
	return
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNamespaces() (namespaces []services.Namespace, err error) {
	cs.fetch(params{
		key: namespacesKey(),
		fetch: func() error {
			namespaces, err = cs.ap.GetNamespaces()
			return err
		},
		useCache: func() error {
			namespaces, err = cs.presence.GetNamespaces()
			return err
		},
		updateCache: func() (keys []string, cerr error) {
			if err := cs.presence.DeleteAllNamespaces(); err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			}
			for _, ns := range namespaces {
				if err := cs.presence.UpsertNamespace(ns); err != nil {
					return nil, trace.Wrap(err)
				}
				keys = append(keys, namespaceKey(ns.Metadata.Name))
			}
			return
		},
	})
	return
}

func nodesKey(namespace string) string {
	return strings.Join([]string{"nodes", namespace}, "/")
}

func nodeKey(namespace, name string) string {
	return strings.Join([]string{"nodes", namespace, name}, "/")
}

// GetNodes is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetNodes(namespace string) (nodes []services.Server, err error) {
	cs.fetch(params{
		key: nodesKey(namespace),
		fetch: func() error {
			nodes, err = cs.ap.GetNodes(namespace)
			return err
		},
		useCache: func() error {
			nodes, err = cs.presence.GetNodes(namespace)
			return err
		},
		updateCache: func() (keys []string, cerr error) {
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
				keys = append(keys, nodeKey(namespace, node.GetName()))
			}
			return
		},
	})
	return
}

func tunnelsKey() string {
	return "tunnels"
}

func tunnelKey(name string) string {
	return strings.Join([]string{"tunnels", name}, "/")
}

// GetReverseTunnels is not using recent cache on purpose
// as it's designed to be called periodically and return fresh data
// at all times when possible
func (cs *CachingAuthClient) GetReverseTunnels() (tunnels []services.ReverseTunnel, err error) {
	err = cs.try(func() error {
		tunnels, err = cs.ap.GetReverseTunnels()
		return err
	})
	if err != nil {
		if trace.IsConnectionProblem(err) {
			tunnels, err = cs.presence.GetReverseTunnels()
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

func proxiesKey() string {
	return "proxies"
}

func proxyKey(name string) string {
	return strings.Join([]string{"proxies", name}, "/")
}

// GetProxies is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetProxies() (proxies []services.Server, err error) {
	cs.fetch(params{
		key: proxiesKey(),
		fetch: func() error {
			proxies, err = cs.ap.GetProxies()
			return err
		},
		useCache: func() error {
			proxies, err = cs.presence.GetProxies()
			return err
		},
		updateCache: func() (keys []string, cerr error) {
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
				keys = append(keys, proxyKey(proxy.GetName()))
			}
			return
		},
	})
	return
}

// GetCertAuthority is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthority(id services.CertAuthID, loadKeys bool) (ca services.CertAuthority, err error) {
	cs.fetch(params{
		key: certKey(id, loadKeys),
		fetch: func() error {
			ca, err = cs.ap.GetCertAuthority(id, loadKeys)
			return err
		},
		useCache: func() error {
			ca, err = cs.trust.GetCertAuthority(id, loadKeys)
			return err
		},
		updateCache: func() (keys []string, cerr error) {
			if err := cs.trust.DeleteCertAuthority(id); err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			}
			cs.setTTL(ca)
			if err := cs.trust.UpsertCertAuthority(ca); err != nil {
				return nil, trace.Wrap(err)
			}
			return
		},
	})
	return
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func certsKey(ct services.CertAuthType, loadKeys bool) string {
	return strings.Join([]string{"cas", string(ct), boolToString(loadKeys)}, "/")
}

func certKey(id services.CertAuthID, loadKeys bool) string {
	return strings.Join([]string{"cas", string(id.Type), boolToString(loadKeys), id.DomainName}, "/")
}

// GetCertAuthorities is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetCertAuthorities(ct services.CertAuthType, loadKeys bool) (cas []services.CertAuthority, err error) {
	cs.fetch(params{
		key: certsKey(ct, loadKeys),
		fetch: func() error {
			cas, err = cs.ap.GetCertAuthorities(ct, loadKeys)
			return err
		},
		useCache: func() error {
			cas, err = cs.trust.GetCertAuthorities(ct, loadKeys)
			return err
		},
		updateCache: func() (keys []string, cerr error) {
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
				keys = append(keys, certKey(ca.GetID(), loadKeys))
			}
			return
		},
	})
	return
}

func usersKey() string {
	return "users"
}

func userKey(username string) string {
	return strings.Join([]string{"users", username}, "/")
}

// GetUsers is a part of auth.AccessPoint implementation
func (cs *CachingAuthClient) GetUsers() (users []services.User, err error) {
	cs.fetch(params{
		key: usersKey(),
		fetch: func() error {
			users, err = cs.ap.GetUsers()
			return err
		},
		useCache: func() error {
			users, err = cs.identity.GetUsers()
			return err
		},
		updateCache: func() (keys []string, cerr error) {
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
				keys = append(keys, userKey(user.GetName()))
			}
			return
		},
	})
	return
}

// GetTunnelConnections is a part of auth.AccessPoint implementation
// GetTunnelConnections are not using recent cache as they are designed
// to be called periodically and always return fresh data
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
// GetAllTunnelConnections are not using recent cache, as they are designed
// to be called periodically and always return fresh data
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

func (cs *CachingAuthClient) setTTL(r services.Resource) {
	if cs.NeverExpires {
		return
	}
	// honor expiry set by user
	if !r.Expiry().IsZero() {
		return
	}
	// set TTL as a global setting
	r.SetTTL(cs.Clock, cs.CacheMaxTTL)
}

type params struct {
	key         string
	fetch       func() error
	useCache    func() error
	updateCache func() ([]string, error)
}

func (cs *CachingAuthClient) getRecentCache(key string) bool {
	cs.Lock()
	defer cs.Unlock()
	// we have to grab write lock here, because
	// Get in recent cache actually expires the value
	_, exists := cs.recentCache.Get(key)
	return exists
}

// setRecentCacheWithLock sets minimum time before the value will be accessed
// from the auth server again
func (cs *CachingAuthClient) setRecentCacheWithLock(key string, value interface{}) {
	if cs.RecentCacheTTL == 0 {
		return
	}
	cs.Lock()
	defer cs.Unlock()
	cs.recentCache.Set(key, value, cs.RecentCacheTTL)
}

// setRecentCacheNoLock sets minimum time before the value will be accessed
// from the auth server again
func (cs *CachingAuthClient) setRecentCacheNoLock(key string, value interface{}) {
	if cs.RecentCacheTTL == 0 {
		return
	}
	cs.recentCache.Set(key, value, cs.RecentCacheTTL)
}

// mdiff is a microseconds diff
func mdiff(start time.Time) float64 {
	return float64(time.Now().Sub(start) / time.Microsecond)
}

func (cs *CachingAuthClient) updateCache(p params) error {
	cs.Lock()
	defer cs.Unlock()
	start := time.Now()
	// cacheKeys could be individual item ids for collection
	// and the full collection name as well to set as remembered
	cacheKeys, err := p.updateCache()
	if err != nil {
		cacheLatencies.Observe(mdiff(start))
		return err
	}
	cs.setRecentCacheNoLock(p.key, true)
	for _, cacheKey := range cacheKeys {
		cs.setRecentCacheNoLock(cacheKey, true)
	}
	cacheLatencies.Observe(mdiff(start))
	return nil
}

func (cs *CachingAuthClient) useCache(p params) error {
	cs.RLock()
	defer cs.RUnlock()
	start := time.Now()
	err := p.useCache()
	cacheLatencies.Observe(mdiff(start))
	return err
}

// fetch function performs cached access to the collection
// using auth server
func (cs *CachingAuthClient) fetch(p params) {
	if cs.getRecentCache(p.key) {
		cs.WithFields(log.Fields{"key": p.key}).Debugf("Recent cache hit.")
		cs.useCache(p)
		return
	}
	cs.WithFields(log.Fields{"key": p.key}).Debugf("Recent cache miss.")
	// try fetching value from the auth server
	err := cs.try(p.fetch)
	if err == nil {
		if err := cs.updateCache(p); err != nil {
			log.Warningf("Failed to update cache: %v.", trace.DebugReport(err))
		}
	}
	// use cache on connection problems
	if trace.IsConnectionProblem(err) {
		cs.useCache(p)
		return
	}
	// cache negative responses, to avoid hitting database all the time
	// if value is not found
	if trace.IsNotFound(err) {
		cs.setRecentCacheWithLock(p.key, err)
	}
	return
}

func (cs *CachingAuthClient) getLastErrorTime() time.Time {
	cs.RLock()
	defer cs.RUnlock()
	return cs.lastErrorTime
}

func (cs *CachingAuthClient) setLastErrorTime(t time.Time) {
	cs.Lock()
	defer cs.Unlock()
	cs.lastErrorTime = t
}

// try calls a given function f and checks for errors. If f() fails, the current
// time is recorded. Future calls to f will be ingored until sufficient time passes
// since th last error
func (cs *CachingAuthClient) try(f func() error) error {
	start := time.Now()
	tooSoon := cs.getLastErrorTime().Add(defaults.NetworkRetryDuration).After(time.Now())
	if tooSoon {
		cs.Warnf("Backoff: using cached value due to recent errors.")
		return trace.ConnectionProblem(fmt.Errorf("backoff"), "backing off due to recent errors")
	}
	accessPointRequests.Inc()
	err := trace.ConvertSystemError(f())
	accessPointLatencies.Observe(mdiff(start))
	if trace.IsConnectionProblem(err) {
		cs.setLastErrorTime(time.Now())
		cs.Warningf("Connection problem: failed connect to the auth servers, using local cache.")
	}
	return err
}
