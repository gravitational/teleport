package cache

import (
	"context"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type provisioningUserStateGetter interface {
	GetUserProvisioningState(context.Context, string) (*provisioningv1.UserState, error)
	ListUserProvisioningStates(context.Context, services.PageToken) ([]*provisioningv1.UserState, services.PageToken, error)
}

type provisioningUserStateExecutor struct{}

func (provisioningUserStateExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*provisioningv1.UserState, error) {
	var nextPage services.PageToken
	var resources []*provisioningv1.UserState
	for {
		var page []*provisioningv1.UserState
		var err error

		if cache == nil {
			panic("Cache is nil")
		}

		if cache.ProvisioningStates == nil {
			panic("Cache ProvisioningStates is nil")
		}

		page, nextPage, err := cache.ProvisioningStates.ListUserProvisioningStates(ctx, nextPage)
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

func (provisioningUserStateExecutor) upsert(ctx context.Context, cache *Cache, resource *provisioningv1.UserState) error {
	_, err := cache.provisioningStatesCache.CreateUserProvisioningState(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.provisioningStatesCache.UpdateUserProvisioningState(ctx, resource)
	}
	return trace.Wrap(err)
}

func (provisioningUserStateExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.provisioningStatesCache.DeleteUserProvisioningState(ctx, resource.GetName()))
}

func (provisioningUserStateExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.provisioningStatesCache.DeleteAllUserProvisioningStates(ctx))
}

func (provisioningUserStateExecutor) getReader(cache *Cache, cacheOK bool) provisioningUserStateGetter {
	if cacheOK {
		return cache.provisioningStatesCache
	}
	return cache.Config.ProvisioningStates
}

func (provisioningUserStateExecutor) isSingleton() bool {
	return false
}

var _ executor[*provisioningv1.UserState, provisioningUserStateGetter] = provisioningUserStateExecutor{}
