package cache

import (
	"context"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
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
	_, err := cache.identityCenterCache.CreateIdentityCenterAccount(ctx, resource.Account)
	if trace.IsAlreadyExists(err) {
		_, err = cache.identityCenterCache.UpdateIdentityCenterAccount(ctx, resource.Account)
	}
	return trace.Wrap(err)
}

func (identityCenterAccountExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeleteIdentityCenterAccount(ctx,
		services.IdentityCenterAccountID(resource.GetName())))
}

func (identityCenterAccountExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAllIdentityCenterAccounts(ctx))
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

type principalAssignmentGetter interface {
	GetPrincipalAssignment(context.Context, services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error)
	ListPrincipalAssignments(context.Context, pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error)
}

type principalAssignmentExecutor struct{}

func (principalAssignmentExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*identitycenterv1.PrincipalAssignment, error) {
	var page pagination.PageRequestToken
	var resources []*identitycenterv1.PrincipalAssignment
	for {
		if cache == nil {
			panic("Cache is nil")
		}

		if cache.ProvisioningStates == nil {
			panic("Cache ProvisioningStates is nil")
		}

		resourcesPage, nextPage, err := cache.IdentityCenter.ListPrincipalAssignments(ctx, page)
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

func (principalAssignmentExecutor) upsert(ctx context.Context, cache *Cache, resource *identitycenterv1.PrincipalAssignment) error {
	_, err := cache.identityCenterCache.CreatePrincipalAssignment(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.identityCenterCache.UpdatePrincipalAssignment(ctx, resource)
	}
	return trace.Wrap(err)
}

func (principalAssignmentExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeletePrincipalAssignment(ctx,
		services.PrincipalAssignmentID(resource.GetName())))
}

func (principalAssignmentExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAllPrincipalAssignments(ctx))
}

func (principalAssignmentExecutor) getReader(cache *Cache, cacheOK bool) principalAssignmentGetter {
	if cacheOK {
		return cache.identityCenterCache
	}
	return cache.Config.IdentityCenter
}

func (principalAssignmentExecutor) isSingleton() bool {
	return false
}

type accountAssignmentGetter interface {
	GetAccountAssignment(context.Context, services.IdentityCenterAccountAssignmentID) (services.IdentityCenterAccountAssignment, error)
	ListAccountAssignments(context.Context, pagination.PageRequestToken) ([]services.IdentityCenterAccountAssignment, pagination.NextPageToken, error)
}

type accountAssignmentExecutor struct{}

func (accountAssignmentExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]services.IdentityCenterAccountAssignment, error) {
	var page pagination.PageRequestToken
	var resources []services.IdentityCenterAccountAssignment
	for {
		if cache == nil {
			panic("Cache is nil")
		}

		if cache.ProvisioningStates == nil {
			panic("Cache ProvisioningStates is nil")
		}

		resourcesPage, nextPage, err := cache.IdentityCenter.ListAccountAssignments(ctx, page)
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

func (accountAssignmentExecutor) upsert(ctx context.Context, cache *Cache, resource services.IdentityCenterAccountAssignment) error {
	_, err := cache.identityCenterCache.CreateAccountAssignment(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.identityCenterCache.UpdateAccountAssignment(ctx, resource)
	}
	return trace.Wrap(err)
}

func (accountAssignmentExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAccountAssignment(ctx,
		services.IdentityCenterAccountAssignmentID(resource.GetName())))
}

func (accountAssignmentExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAllIdentityCenterAccounts(ctx))
}

func (accountAssignmentExecutor) getReader(cache *Cache, cacheOK bool) accountAssignmentGetter {
	if cacheOK {
		return cache.identityCenterCache
	}
	return cache.Config.IdentityCenter
}

func (accountAssignmentExecutor) isSingleton() bool {
	return false
}
