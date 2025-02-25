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

package clusteridcache

import (
	"sync"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// Cache stores cluster IDs indexed by their cluster URIs.
//
// Cluster IDs are required when reporting usage events, but they are not publicly known and can be
// fetched only after logging in to a cluster. Today, most events are sent from the Electron app.
// The Electron app caches cluster IDs on its own. However, sometimes we want to send events
// straight from the tsh daemon, in which case we need to know the ID of a cluster.
//
// Whenever the user logs in and fetches full details of a cluster, the cluster ID gets saved to the
// cache. Later on when tsh daemon wants to send a usage event, it can load the cluster ID from the
// cache.
//
// This cache is never cleared since cluster IDs are saved only for root clusters. Logging in to a
// root cluster overwrites existing ID under the same URI.
//
// TODO(ravicious): Refactor usage reporting to operate on cluster URIs instead of cluster IDs and
// keep the cache only on the side of tsh daemon. Fetch a cluster ID whenever it's first requested
// to avoid an issue with trying to send a usage event before the cluster ID is known.
// https://github.com/gravitational/teleport/issues/23030
type Cache struct {
	m sync.Map
}

// Store stores the cluster ID for the given uri of a root cluster.
func (c *Cache) Store(uri uri.ResourceURI, clusterID string) {
	c.m.Store(uri.String(), clusterID)
}

// Load returns the cluster ID for the given uri of a root cluster.
func (c *Cache) Load(uri uri.ResourceURI) (string, bool) {
	id, ok := c.m.Load(uri.String())

	if !ok {
		return "", false
	}

	return id.(string), true
}
