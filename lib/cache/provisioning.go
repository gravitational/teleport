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

package cache

import (
	"context"

	"github.com/gravitational/trace"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type provisioningStateGetter interface {
	GetProvisioningState(context.Context, services.DownstreamID, services.ProvisioningStateID) (*provisioningv1.PrincipalState, error)
	ListProvisioningStatesForAllDownstreams(context.Context, int, *pagination.PageRequestToken) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error)
}

type provisioningStateExecutor struct{}

func (provisioningStateExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*provisioningv1.PrincipalState, error) {
	if cache == nil {
		return nil, trace.BadParameter("cache is nil")
	}

	if cache.ProvisioningStates == nil {
		return nil, trace.BadParameter("cache provisioning state source is not set")
	}

	var page pagination.PageRequestToken
	var resources []*provisioningv1.PrincipalState
	for {
		var resourcesPage []*provisioningv1.PrincipalState
		var err error

		resourcesPage, nextPage, err := cache.ProvisioningStates.ListProvisioningStatesForAllDownstreams(ctx, 0, &page)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resourcesPage...)

		if nextPage == pagination.EndOfList {
			break
		}
		page.Update(nextPage)
	}
	return resources, nil
}

func (provisioningStateExecutor) upsert(ctx context.Context, cache *Cache, resource *provisioningv1.PrincipalState) error {
	_, err := cache.provisioningStatesCache.UpsertProvisioningState(ctx, resource)
	return trace.Wrap(err)
}

func (provisioningStateExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	unwrapper, ok := resource.(types.Resource153Unwrapper)
	if !ok {
		return trace.BadParameter("resource must implement Resource153Unwrapper: %T", resource)
	}

	principalState, ok := unwrapper.Unwrap().(*provisioningv1.PrincipalState)
	if !ok {
		return trace.BadParameter("wrapped resource must be a PrincipalState: %T", resource)
	}

	principalStateID := principalState.GetMetadata().GetName()
	downstreamID := principalState.GetSpec().GetDownstreamId()
	if principalStateID == "" || downstreamID == "" {
		return trace.BadParameter("malformed PrincipalState")
	}

	err := cache.provisioningStatesCache.DeleteProvisioningState(
		ctx,
		services.DownstreamID(downstreamID),
		services.ProvisioningStateID(principalStateID))
	return trace.Wrap(err)
}

func (provisioningStateExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.provisioningStatesCache.DeleteAllProvisioningStates(ctx))
}

func (provisioningStateExecutor) getReader(cache *Cache, cacheOK bool) provisioningStateGetter {
	if cacheOK {
		return cache.provisioningStatesCache
	}
	return cache.Config.ProvisioningStates
}

func (provisioningStateExecutor) isSingleton() bool {
	return false
}

var _ executor[*provisioningv1.PrincipalState, provisioningStateGetter] = provisioningStateExecutor{}
