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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"

	"github.com/gravitational/trace"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

// SPIFFEFederationReader is an interface that defines the methods for getting
// SPIFFE federations. This is returned as the reader for the SPIFFEFederations
// collection but is also used by the executor to read the full list of
// SPIFFE Federations on initialization.
type SPIFFEFederationReader interface {
	ListSPIFFEFederations(ctx context.Context, pageSize int, nextToken string) ([]*machineidv1.SPIFFEFederation, string, error)
	GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error)
}

// spiffeFederationCacher is used for storing and retrieving SPIFFE federations
// from the cache's local backend.
type spiffeFederationCacher interface {
	SPIFFEFederationReader
	UpsertSPIFFEFederation(ctx context.Context, federation *machineidv1.SPIFFEFederation) (*machineidv1.SPIFFEFederation, error)
	DeleteSPIFFEFederation(ctx context.Context, name string) error
	DeleteAllSPIFFEFederations(ctx context.Context) error
}

type spiffeFederationExecutor struct{}

var _ executor[*machineidv1.SPIFFEFederation, SPIFFEFederationReader] = spiffeFederationExecutor{}

func (spiffeFederationExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*machineidv1.SPIFFEFederation, error) {
	var out []*machineidv1.SPIFFEFederation
	var nextToken string
	for {
		var page []*machineidv1.SPIFFEFederation
		var err error

		page, nextToken, err = cache.Config.SPIFFEFederations.ListSPIFFEFederations(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (spiffeFederationExecutor) upsert(ctx context.Context, cache *Cache, resource *machineidv1.SPIFFEFederation) error {
	_, err := cache.spiffeFederationCache.UpsertSPIFFEFederation(ctx, resource)
	return trace.Wrap(err)
}

func (spiffeFederationExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.spiffeFederationCache.DeleteAllSPIFFEFederations(ctx))
}

func (spiffeFederationExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.spiffeFederationCache.DeleteSPIFFEFederation(ctx, resource.GetName()))
}

func (spiffeFederationExecutor) isSingleton() bool { return false }

func (spiffeFederationExecutor) getReader(cache *Cache, cacheOK bool) SPIFFEFederationReader {
	if cacheOK {
		return cache.spiffeFederationCache
	}
	return cache.Config.SPIFFEFederations
}

// ListSPIFFEFederations returns a paginated list of SPIFFE federations
func (c *Cache) ListSPIFFEFederations(ctx context.Context, pageSize int, nextToken string) ([]*machineidv1.SPIFFEFederation, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSPIFFEFederations")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.spiffeFederations)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	out, nextKey, err := rg.reader.ListSPIFFEFederations(ctx, pageSize, nextToken)
	return out, nextKey, trace.Wrap(err)
}

// GetSPIFFEFederation returns a single SPIFFE federation by name
func (c *Cache) GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSPIFFEFederation")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.spiffeFederations)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	out, err := rg.reader.GetSPIFFEFederation(ctx, name)
	return out, trace.Wrap(err)
}
