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

package remoteclientcache

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

// CacheI defines an interface for proxy client cache.
type CacheI interface {
	// Get returns a proxy client from the cache if there is one,
	// otherwise it dials the remote server.
	Get(ctx context.Context, clusterURI uri.ResourceURI) (*client.ProxyClient, error)
	// InvalidateForRootCluster closes and removes clients from the cache
	// for the root cluster and its leaf clusters.
	InvalidateForRootCluster(rootClusterURI uri.ResourceURI) error
	// Close closes and removes all clients.
	Close() error
}

// Cache stores remote clients keyed by cluster URI.
// Safe for concurrent access.
// Closes all clients and wipes the cache on Close.
type Cache struct {
	clusters.Resolver
	mu  sync.Mutex
	log *logrus.Entry
	// clients keep mapping between cluster URI
	// (both root and leaf) and proxy clients
	clients map[uri.ResourceURI]*client.ProxyClient
	// group prevents duplicate requests to create remote clients
	// for a given cluster URI
	group singleflight.Group
}

func New(c Config) *Cache {
	return &Cache{
		log:      c.Log,
		clients:  make(map[uri.ResourceURI]*client.ProxyClient),
		Resolver: c.Resolver,
	}
}

func (c *Cache) Get(ctx context.Context, clusterURI uri.ResourceURI) (*client.ProxyClient, error) {
	cltI, err, _ := c.group.Do(clusterURI.String(), func() (interface{}, error) {
		cluster, clusterClient, err := c.ResolveCluster(clusterURI)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Check if we already have a cached client for this cluster and a valid cert.
		// Theoretically, we don't have to check the validity
		// because auth server will disconnect expired clients.
		// However, by default, it isn't done immediately
		// (until it is enabled with disconnect_expired_cert on cluster) and to keep
		// the previous behavior, we need to check the cert.
		//
		// If the cert is expired, we will remove the client and make an attempt
		// to connect to proxy which will fail with an appropriate error.
		proxyClient := c.getFromCache(clusterURI)
		if proxyClient != nil {
			if cluster.Connected() {
				return proxyClient, nil
			}
			err := c.InvalidateForRootCluster(clusterURI.GetRootClusterURI())
			if err != nil {
				c.log.WithError(err).Errorf("Failed to invalidate expired remote client for %q.", clusterURI)
			}
		}

		newProxyClient, err := clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// We'll save the remote client in the cache, so we don't have to
		// build a new connection next time.
		// All remote clients will be closed when the daemon exits.
		err = c.addToCache(clusterURI, newProxyClient)
		if err != nil {
			c.log.WithError(err).Errorf("Failed to add remote client for %q to cache.", clusterURI)
		} else {
			c.log.Infof("Added remote client for %q to cache.", clusterURI)
		}

		return newProxyClient, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, ok := cltI.(*client.ProxyClient)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T received for proxy client", cltI)
	}

	return clt, nil
}

func (c *Cache) InvalidateForRootCluster(rootClusterURI uri.ResourceURI) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	errors := make([]error, 0, len(c.clients))
	deleted := make([]uri.ResourceURI, 0, len(c.clients))

	for resourceURI, clt := range c.clients {
		if resourceURI.GetRootClusterURI() == rootClusterURI {
			err := clt.Close()
			if err != nil {
				errors = append(errors, err)
			}
			deleted = append(deleted, resourceURI.GetClusterURI())
			delete(c.clients, resourceURI)
		}
	}

	c.log.Infof("Invalidated cached remote clients for root cluster %q: %v", rootClusterURI, deleted)

	return trace.NewAggregate(errors...)

}

func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	errors := make([]error, 0, len(c.clients))
	for _, clt := range c.clients {
		errors = append(errors, clt.Close())
	}
	c.clients = make(map[uri.ResourceURI]*client.ProxyClient)

	return trace.NewAggregate(errors...)
}

func (c *Cache) addToCache(clusterURI uri.ResourceURI, proxyClient *client.ProxyClient) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	if c.clients[clusterURI] != nil {
		err = c.clients[clusterURI].Close()
	}
	c.clients[clusterURI] = proxyClient

	go func() {
		err := proxyClient.Client.Wait()
		if err != nil {
			c.mu.Lock()
			defer c.mu.Unlock()

			delete(c.clients, clusterURI)
			c.log.WithError(err).Infof("Remote client to %q has been closed and removed from cache.", clusterURI)
		}
	}()
	return trace.Wrap(err)
}

func (c *Cache) getFromCache(clusterURI uri.ResourceURI) *client.ProxyClient {
	c.mu.Lock()
	defer c.mu.Unlock()

	remoteClt := c.clients[clusterURI]
	return remoteClt
}

type Config struct {
	clusters.Resolver
	Log *logrus.Entry
}
