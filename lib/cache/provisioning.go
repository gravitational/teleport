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

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type provisioningStateGetter interface {
	GetProvisioningState(context.Context, services.ProvisioningStateID) (*provisioningv1.PrincipalState, error)
	ListProvisioningStates(context.Context, services.PageToken) ([]*provisioningv1.PrincipalState, services.PageToken, error)
}

type provisioningStateExecutor struct{}

func (provisioningStateExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*provisioningv1.PrincipalState, error) {
	var nextPage services.PageToken
	var resources []*provisioningv1.PrincipalState
	for {
		var page []*provisioningv1.PrincipalState
		var err error

		if cache == nil {
			panic("Cache is nil")
		}

		if cache.ProvisioningStates == nil {
			panic("Cache ProvisioningStates is nil")
		}

		page, nextPage, err := cache.ProvisioningStates.ListProvisioningStates(ctx, nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, page...)

		if nextPage == services.EndOfList {
			break
		}
	}
	return resources, nil
}

func (provisioningStateExecutor) upsert(ctx context.Context, cache *Cache, resource *provisioningv1.PrincipalState) error {
	_, err := cache.provisioningStatesCache.CreateProvisioningState(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.provisioningStatesCache.UpdateProvisioningState(ctx, resource)
	}
	return trace.Wrap(err)
}

func (provisioningStateExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.provisioningStatesCache.DeleteProvisioningState(ctx,
		services.ProvisioningStateID(resource.GetName())))
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
