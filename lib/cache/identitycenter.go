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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type identityCenterAccountGetter interface {
	GetIdentityCenterAccount(context.Context, string) (*identitycenterv1.Account, error)
	ListIdentityCenterAccounts(context.Context, int, string) ([]*identitycenterv1.Account, string, error)
}

type identityCenterAccountExecutor struct{}

func (identityCenterAccountExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*identitycenterv1.Account, error) {
	return stream.Collect(clientutils.Resources(ctx, cache.IdentityCenter.ListIdentityCenterAccounts))
}

func (identityCenterAccountExecutor) upsert(ctx context.Context, cache *Cache, acct *identitycenterv1.Account) error {
	_, err := cache.identityCenterCache.UpsertIdentityCenterAccount(ctx, acct)
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

var _ executor[
	*identitycenterv1.Account,
	identityCenterAccountGetter,
] = identityCenterAccountExecutor{}

type identityCenterPrincipalAssignmentGetter interface {
	GetPrincipalAssignment(context.Context, services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error)
	ListPrincipalAssignments(context.Context, int, string) ([]*identitycenterv1.PrincipalAssignment, string, error)
	ListPrincipalAssignments2(context.Context, int, string) ([]*identitycenterv1.PrincipalAssignment, string, error)
}

type identityCenterPrincipalAssignmentExecutor struct{}

var _ executor[
	*identitycenterv1.PrincipalAssignment,
	identityCenterPrincipalAssignmentGetter,
] = identityCenterPrincipalAssignmentExecutor{}

func (identityCenterPrincipalAssignmentExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*identitycenterv1.PrincipalAssignment, error) {
	return stream.Collect(clientutils.Resources(ctx, cache.IdentityCenter.ListPrincipalAssignments))
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
	return trace.Wrap(cache.identityCenterCache.DeleteAllIdentityCenterAccounts(ctx))
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
	GetIdentityCenterAccountAssignment(context.Context, string) (*identitycenterv1.AccountAssignment, error)
	ListIdentityCenterAccountAssignments(context.Context, int, string) ([]*identitycenterv1.AccountAssignment, string, error)
}

type identityCenterAccountAssignmentExecutor struct{}

func (identityCenterAccountAssignmentExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*identitycenterv1.AccountAssignment, error) {
	return stream.Collect(clientutils.Resources(ctx, cache.IdentityCenter.ListIdentityCenterAccountAssignments))
}

func (identityCenterAccountAssignmentExecutor) upsert(ctx context.Context, cache *Cache, resource *identitycenterv1.AccountAssignment) error {
	_, err := cache.identityCenterCache.UpsertAccountAssignment(ctx, resource)
	return trace.Wrap(err)
}

func (identityCenterAccountAssignmentExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAccountAssignment(ctx,
		services.IdentityCenterAccountAssignmentID(resource.GetName())))
}

func (identityCenterAccountAssignmentExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.identityCenterCache.DeleteAllAccountAssignments(ctx))
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
	*identitycenterv1.AccountAssignment,
	identityCenterAccountAssignmentGetter,
] = identityCenterAccountAssignmentExecutor{}
