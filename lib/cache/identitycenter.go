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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type identityCenterAccountIndex string

const identityCenterAccountNameIndex identityCenterAccountIndex = "name"

func newIdentityCenterAccountCollection(ic services.IdentityCenter, w types.WatchKind) (*collection[*identitycenterv1.Account, identityCenterAccountIndex], error) {
	if ic == nil {
		return nil, trace.BadParameter("missing parameter IdentityCenter")
	}

	return &collection[*identitycenterv1.Account, identityCenterAccountIndex]{
		store: newStore(map[identityCenterAccountIndex]func(*identitycenterv1.Account) string{
			identityCenterAccountNameIndex: func(r *identitycenterv1.Account) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*identitycenterv1.Account, error) {
			var pageToken pagination.PageRequestToken
			var accounts []*identitycenterv1.Account
			for {
				resp, nextToken, err := ic.ListIdentityCenterAccounts(ctx, 0, &pageToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				for _, item := range resp {
					accounts = append(accounts, item.Account)
				}
				if nextToken == "" {
					break
				}
				pageToken.Update(nextToken)
			}
			return accounts, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *identitycenterv1.Account {
			return &identitycenterv1.Account{
				Kind:    hdr.Kind,
				SubKind: hdr.SubKind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

func (c *Cache) GetIdentityCenterAccount(ctx context.Context, name services.IdentityCenterAccountID) (services.IdentityCenterAccount, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetIdentityCenterAccount")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.identityCenterAccounts)
	if err != nil {
		return services.IdentityCenterAccount{}, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		account, err := c.Config.IdentityCenter.GetIdentityCenterAccount(ctx, name)
		return account, trace.Wrap(err)
	}

	account, err := rg.store.get(identityCenterAccountNameIndex, string(name))
	if err != nil {
		return services.IdentityCenterAccount{}, trace.Wrap(err)
	}

	return services.IdentityCenterAccount{Account: utils.CloneProtoMsg(account)}, nil
}

func (c *Cache) ListIdentityCenterAccounts(ctx context.Context, pageSize int, token *pagination.PageRequestToken) ([]services.IdentityCenterAccount, pagination.NextPageToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListIdentityCenterAccounts")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.identityCenterAccounts)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		accounts, next, err := c.Config.IdentityCenter.ListIdentityCenterAccounts(ctx, pageSize, token)
		return accounts, next, trace.Wrap(err)
	}

	if pageSize == 0 {
		pageSize = 100
	}

	var accounts []services.IdentityCenterAccount
	startKey, err := token.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	for account := range rg.store.resources(identityCenterAccountNameIndex, startKey, "") {
		if len(accounts) == pageSize {
			return accounts, pagination.NextPageToken(account.Metadata.GetName()), nil
		}

		accounts = append(accounts, services.IdentityCenterAccount{Account: utils.CloneProtoMsg(account)})

	}
	return accounts, "", nil
}

type identityCenterAccountAssignmentIndex string

const identityCenterAccountAssignmentNameIndex identityCenterAccountAssignmentIndex = "name"

func newIdentityCenterAccountAssignmentCollection(ic services.IdentityCenter, w types.WatchKind) (*collection[*identitycenterv1.AccountAssignment, identityCenterAccountAssignmentIndex], error) {
	if ic == nil {
		return nil, trace.BadParameter("missing parameter IdentityCenter")
	}

	return &collection[*identitycenterv1.AccountAssignment, identityCenterAccountAssignmentIndex]{
		store: newStore(map[identityCenterAccountAssignmentIndex]func(*identitycenterv1.AccountAssignment) string{
			identityCenterAccountAssignmentNameIndex: func(r *identitycenterv1.AccountAssignment) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*identitycenterv1.AccountAssignment, error) {
			var pageToken pagination.PageRequestToken
			var accounts []*identitycenterv1.AccountAssignment
			for {
				resp, nextToken, err := ic.ListAccountAssignments(ctx, 0, &pageToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				for _, item := range resp {
					accounts = append(accounts, item.AccountAssignment)
				}
				if nextToken == "" {
					break
				}
				pageToken.Update(nextToken)
			}
			return accounts, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *identitycenterv1.AccountAssignment {
			return &identitycenterv1.AccountAssignment{
				Kind:    hdr.Kind,
				SubKind: hdr.SubKind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

func (c *Cache) GetAccountAssignment(ctx context.Context, id services.IdentityCenterAccountAssignmentID) (services.IdentityCenterAccountAssignment, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccountAssignment")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.identityCenterAccountAssignments)
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		assignment, err := c.Config.IdentityCenter.GetAccountAssignment(ctx, id)
		return assignment, trace.Wrap(err)
	}

	assignment, err := rg.store.get(identityCenterAccountAssignmentNameIndex, string(id))
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err)
	}

	return services.IdentityCenterAccountAssignment{AccountAssignment: assignment}, nil
}

// ListAccountAssignments fetches a paginated list of IdentityCenter Account Assignments
func (c *Cache) ListAccountAssignments(ctx context.Context, pageSize int, pageToken *pagination.PageRequestToken) ([]services.IdentityCenterAccountAssignment, pagination.NextPageToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccountAssignments")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.identityCenterAccountAssignments)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		assignment, next, err := c.Config.IdentityCenter.ListAccountAssignments(ctx, pageSize, pageToken)
		return assignment, next, trace.Wrap(err)
	}

	if pageSize == 0 {
		pageSize = 100
	}

	token, err := pageToken.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err, "extracting page token")
	}

	var assignments []services.IdentityCenterAccountAssignment
	for assignment := range rg.store.resources(identityCenterAccountAssignmentNameIndex, token, "") {
		if len(assignments) == pageSize {
			return assignments, pagination.NextPageToken(assignment.GetMetadata().Name), nil
		}

		assignments = append(assignments, services.IdentityCenterAccountAssignment{
			AccountAssignment: utils.CloneProtoMsg(assignment),
		})

	}
	return assignments, "", nil

}

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
