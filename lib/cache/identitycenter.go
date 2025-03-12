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

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type identityCenterAccountGetter interface {
	GetIdentityCenterAccount(context.Context, services.IdentityCenterAccountID) (services.IdentityCenterAccount, error)
	ListIdentityCenterAccounts(context.Context, int, *pagination.PageRequestToken) ([]services.IdentityCenterAccount, pagination.NextPageToken, error)
}

type identityCenterAccountExecutor struct{}

func (identityCenterAccountExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]services.IdentityCenterAccount, error) {
	var pageToken pagination.PageRequestToken
	var resources []services.IdentityCenterAccount
	for {
		resourcesPage, nextPage, err := cache.IdentityCenter.ListIdentityCenterAccounts(ctx, 0, &pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resourcesPage...)

		if nextPage == pagination.EndOfList {
			break
		}
		pageToken.Update(nextPage)
	}
	return resources, nil
}

func (identityCenterAccountExecutor) upsert(ctx context.Context, cache *Cache, resource services.IdentityCenterAccount) error {
	_, err := cache.identityCenterCache.UpsertIdentityCenterAccount(ctx, resource)
	return trace.Wrap(err)
}

func (identityCenterAccountExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeleteIdentityCenterAccount(ctx,
		services.IdentityCenterAccountID(resource.GetName())))
}

func (identityCenterAccountExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	_, err := cache.identityCenterCache.DeleteAllIdentityCenterAccounts(ctx, &identitycenterv1.DeleteAllIdentityCenterAccountsRequest{})
	return trace.Wrap(err)
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

var _ executor[
	services.IdentityCenterAccount,
	identityCenterAccountGetter,
] = identityCenterAccountExecutor{}

type identityCenterPrincipalAssignmentGetter interface {
	GetPrincipalAssignment(context.Context, services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error)
	ListPrincipalAssignments(context.Context, int, *pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error)
}

type identityCenterPrincipalAssignmentExecutor struct{}

var _ executor[
	*identitycenterv1.PrincipalAssignment,
	identityCenterPrincipalAssignmentGetter,
] = identityCenterPrincipalAssignmentExecutor{}

func (identityCenterPrincipalAssignmentExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*identitycenterv1.PrincipalAssignment, error) {
	var pageToken pagination.PageRequestToken
	var resources []*identitycenterv1.PrincipalAssignment
	for {
		resourcesPage, nextPage, err := cache.IdentityCenter.ListPrincipalAssignments(ctx, 0, &pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resourcesPage...)

		if nextPage == pagination.EndOfList {
			break
		}
		pageToken.Update(nextPage)
	}
	return resources, nil
}

func (identityCenterPrincipalAssignmentExecutor) upsert(ctx context.Context, cache *Cache, resource *identitycenterv1.PrincipalAssignment) error {
	_, err := cache.identityCenterCache.UpsertPrincipalAssignment(ctx, resource)
	return trace.Wrap(err)
}

func (identityCenterPrincipalAssignmentExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeletePrincipalAssignment(ctx,
		services.PrincipalAssignmentID(resource.GetName())))
}

func (identityCenterPrincipalAssignmentExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	_, err := cache.identityCenterCache.DeleteAllPrincipalAssignments(ctx, &identitycenterv1.DeleteAllPrincipalAssignmentsRequest{})
	return trace.Wrap(err)
}

func (identityCenterPrincipalAssignmentExecutor) getReader(cache *Cache, cacheOK bool) identityCenterPrincipalAssignmentGetter {
	if cacheOK {
		return cache.identityCenterCache
	}
	return cache.Config.IdentityCenter
}

func (identityCenterPrincipalAssignmentExecutor) isSingleton() bool {
	return false
}

type identityCenterAccountAssignmentGetter interface {
	GetAccountAssignment(context.Context, services.IdentityCenterAccountAssignmentID) (services.IdentityCenterAccountAssignment, error)
	ListAccountAssignments(context.Context, int, *pagination.PageRequestToken) ([]services.IdentityCenterAccountAssignment, pagination.NextPageToken, error)
}

type identityCenterAccountAssignmentExecutor struct{}

func (identityCenterAccountAssignmentExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]services.IdentityCenterAccountAssignment, error) {
	var pageToken pagination.PageRequestToken
	var resources []services.IdentityCenterAccountAssignment
	for {
		resourcesPage, nextPage, err := cache.IdentityCenter.ListAccountAssignments(ctx, 0, &pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resourcesPage...)

		if nextPage == pagination.EndOfList {
			break
		}
		pageToken.Update(nextPage)
	}
	return resources, nil
}

func (identityCenterAccountAssignmentExecutor) upsert(ctx context.Context, cache *Cache, resource services.IdentityCenterAccountAssignment) error {
	_, err := cache.identityCenterCache.UpsertAccountAssignment(ctx, resource)
	return trace.Wrap(err)
}

func (identityCenterAccountAssignmentExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAccountAssignment(ctx,
		services.IdentityCenterAccountAssignmentID(resource.GetName())))
}

func (identityCenterAccountAssignmentExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	_, err := cache.identityCenterCache.DeleteAllIdentityCenterAccounts(ctx, &identitycenterv1.DeleteAllIdentityCenterAccountsRequest{})
	return trace.Wrap(err)
}

func (identityCenterAccountAssignmentExecutor) getReader(cache *Cache, cacheOK bool) identityCenterAccountAssignmentGetter {
	if cacheOK {
		return cache.identityCenterCache
	}
	return cache.Config.IdentityCenter
}

func (identityCenterAccountAssignmentExecutor) isSingleton() bool {
	return false
}

var _ executor[
	services.IdentityCenterAccountAssignment,
	identityCenterAccountAssignmentGetter,
] = identityCenterAccountAssignmentExecutor{}
