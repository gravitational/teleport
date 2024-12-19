// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clientcache

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
)

// Cache stores clients keyed by profile name and leaf cluster name.
// Safe for concurrent access.
// Closes all clients and wipes the cache on Clear.
type Cache struct {
	cfg Config
	mu  sync.RWMutex
	// clients keeps a mapping from key (profile name and leaf cluster name) to cluster client.
	clients map[key]*client.ClusterClient
	// group prevents duplicate requests to create clients for a given cluster.
	group singleflight.Group
}

// NewClientFunc is a function that will return a new [*client.TeleportClient] for a given profile and leaf
// cluster. [leafClusterName] may be empty, in which case implementations should return a client for the root cluster.
type NewClientFunc func(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error)

// RetryWithReloginFunc is a function that should call [fn], and if it fails with an error that may be
// resolved with a cluster relogin, attempts the relogin and calls [fn] again if the relogin is successful.
type RetryWithReloginFunc func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error

// Config describes the client cache configuration.
type Config struct {
	NewClientFunc        NewClientFunc
	RetryWithReloginFunc RetryWithReloginFunc
	Logger               *slog.Logger
}

func (c *Config) checkAndSetDefaults() error {
	if c.NewClientFunc == nil {
		return trace.BadParameter("NewClientFunc is required")
	}
	if c.RetryWithReloginFunc == nil {
		return trace.BadParameter("RetryWithReloginFunc is required")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "clientcache")
	}
	return nil
}

type key struct {
	profile     string
	leafCluster string
}

func (k key) String() string {
	if k.leafCluster != "" {
		return k.profile + "/" + k.leafCluster
	}
	return k.profile
}

// New creates an instance of Cache.
func New(c Config) (*Cache, error) {
	if err := c.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Cache{
		cfg:     c,
		clients: make(map[key]*client.ClusterClient),
	}, nil
}

// Get returns a client from the cache if there is one, otherwise it dials the remote server.
// The caller should not close the returned client.
func (c *Cache) Get(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	k := key{profile: profileName, leafCluster: leafClusterName}
	groupClt, err, _ := c.group.Do(k.String(), func() (any, error) {
		if fromCache := c.getFromCache(k); fromCache != nil {
			c.cfg.Logger.DebugContext(ctx, "Retrieved client from cache", "cluster", k)
			return fromCache, nil
		}

		tc, err := c.cfg.NewClientFunc(ctx, profileName, leafClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var newClient *client.ClusterClient
		if err := c.cfg.RetryWithReloginFunc(ctx, tc, func() error {
			clt, err := tc.ConnectToCluster(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			newClient = clt
			return nil
		}); err != nil {
			return nil, trace.Wrap(err)
		}

		// Save the client in the cache, so we don't have to build a new connection next time.
		c.addToCache(k, newClient)

		c.cfg.Logger.InfoContext(ctx, "Added client to cache", "cluster", k)

		return newClient, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, ok := groupClt.(*client.ClusterClient)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T received for cluster client", groupClt)
	}

	return clt, nil
}

// ClearForRoot closes and removes clients from the cache for the root cluster and its leaf clusters.
func (c *Cache) ClearForRoot(profileName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var (
		errors  []error
		deleted []string
	)

	for k, clt := range c.clients {
		if k.profile == profileName {
			if err := clt.Close(); err != nil {
				errors = append(errors, err)
			}
			deleted = append(deleted, k.String())
			delete(c.clients, k)
		}
	}

	c.cfg.Logger.InfoContext(context.Background(), "Invalidated cached clients for root cluster",
		"cluster", profileName,
		"clients", deleted,
	)

	return trace.NewAggregate(errors...)

}

// Clear closes and removes all clients.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error
	for _, clt := range c.clients {
		if err := clt.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	clear(c.clients)

	return trace.NewAggregate(errors...)
}

func (c *Cache) addToCache(k key, clusterClient *client.ClusterClient) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clients[k] = clusterClient
}

func (c *Cache) getFromCache(k key) *client.ClusterClient {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clt := c.clients[k]
	return clt
}

// NoCache is a client cache implementation that returns a new client
// on each call to Get.
//
// ClearForRoot and Clear still work as expected.
type NoCache struct {
	mu            sync.Mutex
	newClientFunc NewClientFunc
	clients       []noCacheClient
}

type noCacheClient struct {
	k      key
	client *client.ClusterClient
}

func NewNoCache(newClientFunc NewClientFunc) *NoCache {
	return &NoCache{
		newClientFunc: newClientFunc,
	}
}

func (c *NoCache) Get(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	clusterClient, err := c.newClientFunc(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newClient, err := clusterClient.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mu.Lock()
	c.clients = append(c.clients, noCacheClient{
		k:      key{profile: profileName, leafCluster: leafClusterName},
		client: newClient,
	})
	c.mu.Unlock()

	return newClient, nil
}

func (c *NoCache) ClearForRoot(profileName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var (
		errors []error
	)

	c.clients = slices.DeleteFunc(c.clients, func(ncc noCacheClient) bool {
		belongsToCluster := ncc.k.profile == profileName

		if belongsToCluster {
			if err := ncc.client.Close(); err != nil {
				errors = append(errors, err)
			}
		}

		return belongsToCluster
	})

	return trace.NewAggregate(errors...)
}

func (c *NoCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error
	for _, ncc := range c.clients {
		if err := ncc.client.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	c.clients = nil

	return trace.NewAggregate(errors...)
}
