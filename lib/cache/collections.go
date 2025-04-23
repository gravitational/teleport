// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// collectionHandler is used by the [Cache] to seed the initial
// data and process events for a particular resource.
type collectionHandler interface {
	// fetch fetches resources and returns a function which will apply said resources to the cache.
	// fetch *must* not mutate cache state outside of the apply function.
	// The provided cacheOK flag indicates whether this collection will be included in the cache generation that is
	// being prepared. If cacheOK is false, fetch shouldn't fetch any resources, but the apply function that it
	// returns must still delete resources from the backend.
	fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// onDelete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to clear.
	onDelete(t types.Resource) error
	// onPut will update a single target resource from the cache
	onPut(t types.Resource) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// collections is the group of resource [collection]s
// that the [Cache] supports.
type collections struct {
	byKind map[resourceKind]collectionHandler

	staticTokens    *collection[types.StaticTokens]
	certAuthorities *collection[types.CertAuthority]
	users           *collection[types.User]
}

// setupCollections ensures that the appropriate [collection] is
// initialized for all provided [types.WatcKind]s. An error is
// returned if a [types.WatchKind] has no associated [collection].
func setupCollections(c Config) (*collections, error) {
	out := &collections{
		byKind: make(map[resourceKind]collectionHandler, 1),
	}

	for _, watch := range c.Watches {
		resourceKind := resourceKindFromWatchKind(watch)

		switch watch.Kind {
		case types.KindStaticTokens:
			collect, err := newStaticTokensCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.staticTokens = collect
			out.byKind[resourceKind] = out.staticTokens
		case types.KindCertAuthority:
			collect, err := newCertAuthorityCollection(c.Trust, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.certAuthorities = collect
			out.byKind[resourceKind] = out.certAuthorities
		case types.KindUser:
			collect, err := newUserCollection(c.Users, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.users = collect
			out.byKind[resourceKind] = out.users
		}
	}

	return out, nil
}
