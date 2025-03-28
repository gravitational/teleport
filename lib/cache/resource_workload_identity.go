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

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

// WorkloadIdentityReader is an interface that defines the methods for getting
// WorkloadIdentity. This is returned as the reader for the WorkloadIdentity
// collection but is also used by the executor to read the full list of
// WorkloadIdentity on initialization.
type WorkloadIdentityReader interface {
	ListWorkloadIdentities(ctx context.Context, pageSize int, nextToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error)
	GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error)
}

// workloadIdentityCacher is used for storing and retrieving WorkloadIdentity
// from the cache's local backend.
type workloadIdentityCacher interface {
	WorkloadIdentityReader
	UpsertWorkloadIdentity(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, error)
	DeleteWorkloadIdentity(ctx context.Context, name string) error
	DeleteAllWorkloadIdentities(ctx context.Context) error
}

type workloadIdentityExecutor struct{}

var _ executor[*workloadidentityv1pb.WorkloadIdentity, WorkloadIdentityReader] = workloadIdentityExecutor{}

func (workloadIdentityExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
	var out []*workloadidentityv1pb.WorkloadIdentity
	var nextToken string
	for {
		var page []*workloadidentityv1pb.WorkloadIdentity
		var err error

		const defaultPageSize = 0
		page, nextToken, err = cache.Config.WorkloadIdentity.ListWorkloadIdentities(ctx, defaultPageSize, nextToken)
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

func (workloadIdentityExecutor) upsert(ctx context.Context, cache *Cache, resource *workloadidentityv1pb.WorkloadIdentity) error {
	_, err := cache.workloadIdentityCache.UpsertWorkloadIdentity(ctx, resource)
	return trace.Wrap(err)
}

func (workloadIdentityExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.workloadIdentityCache.DeleteAllWorkloadIdentities(ctx))
}

func (workloadIdentityExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.workloadIdentityCache.DeleteWorkloadIdentity(ctx, resource.GetName()))
}

func (workloadIdentityExecutor) isSingleton() bool { return false }

func (workloadIdentityExecutor) getReader(cache *Cache, cacheOK bool) WorkloadIdentityReader {
	if cacheOK {
		return cache.workloadIdentityCache
	}
	return cache.Config.WorkloadIdentity
}

// ListWorkloadIdentities returns a paginated list of WorkloadIdentity resources.
func (c *Cache) ListWorkloadIdentities(ctx context.Context, pageSize int, nextToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWorkloadIdentities")
	defer span.End()

	rg, err := readLegacyCollectionCache(c, c.legacyCacheCollections.workloadIdentity)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	out, nextKey, err := rg.reader.ListWorkloadIdentities(ctx, pageSize, nextToken)
	return out, nextKey, trace.Wrap(err)
}

// GetWorkloadIdentity returns a single WorkloadIdentity by name
func (c *Cache) GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWorkloadIdentity")
	defer span.End()

	rg, err := readLegacyCollectionCache(c, c.legacyCacheCollections.workloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	out, err := rg.reader.GetWorkloadIdentity(ctx, name)
	return out, trace.Wrap(err)
}
