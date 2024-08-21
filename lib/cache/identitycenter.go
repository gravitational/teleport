package cache

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
	"github.com/gravitational/trace"
)

type identityCenterAccountGetter interface {
	GetIdentityCenterAccount(context.Context, services.IdentityCenterAccountID) (services.IdentityCenterAccount, error)
	ListIdentityCenterAccounts(context.Context, pagination.PageRequestToken) ([]services.IdentityCenterAccount, pagination.NextPageToken, error)
}

type identityCenterAccountExecutor struct{}

func (identityCenterAccountExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]services.IdentityCenterAccount, error) {
	var page pagination.PageRequestToken
	var resources []services.IdentityCenterAccount
	for {
		if cache == nil {
			panic("Cache is nil")
		}

		if cache.ProvisioningStates == nil {
			panic("Cache ProvisioningStates is nil")
		}

		resourcesPage, nextPage, err := cache.IdentityCenter.ListIdentityCenterAccounts(ctx, page)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resourcesPage...)

		if nextPage == pagination.EndOfList {
			break
		}
		page = pagination.PageRequestToken(nextPage)
	}
	return resources, nil
}

func (identityCenterAccountExecutor) upsert(ctx context.Context, cache *Cache, resource services.IdentityCenterAccount) error {
	_, err := cache.IdentityCenter.CreateIdentityCenterAccount(ctx, resource.Account)
	if trace.IsAlreadyExists(err) {
		_, err = cache.IdentityCenter.UpdateIdentityCenterAccount(ctx, resource.Account)
	}
	return trace.Wrap(err)
}

func (identityCenterAccountExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.IdentityCenter.DeleteIdentityCenterAccount(ctx,
		services.IdentityCenterAccountID(resource.GetName())))
}

func (identityCenterAccountExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.IdentityCenter.DeleteAllIdentityCenterAccounts(ctx))
}

func (identityCenterAccountExecutor) getReader(cache *Cache, cacheOK bool) identityCenterAccountGetter {
	if cacheOK {
		return cache.identityCenterCache
	}
	return cache.Config.IdentityCenter
}

func (identityCenterAccountExecutor) isSingleton() bool {
	return false
}

var _ executor[services.IdentityCenterAccount, identityCenterAccountGetter] = identityCenterAccountExecutor{}
