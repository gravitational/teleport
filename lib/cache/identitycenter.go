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
	"google.golang.org/protobuf/proto"

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
		store: newStore(
			proto.CloneOf[*identitycenterv1.Account],
			map[identityCenterAccountIndex]func(*identitycenterv1.Account) string{
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
		store: newStore(
			proto.CloneOf[*identitycenterv1.AccountAssignment],
			map[identityCenterAccountAssignmentIndex]func(*identitycenterv1.AccountAssignment) string{
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

type identityCenterPrincipalAssignmentIndex string

const identityCenterPrincipalAssignmentNameIndex identityCenterPrincipalAssignmentIndex = "name"

func newIdentityCenterPrincipalAssignmentCollection(upstream services.IdentityCenter, w types.WatchKind) (*collection[*identitycenterv1.PrincipalAssignment, identityCenterPrincipalAssignmentIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter IdentityCenter")
	}

	return &collection[*identitycenterv1.PrincipalAssignment, identityCenterPrincipalAssignmentIndex]{
		store: newStore(
			proto.CloneOf[*identitycenterv1.PrincipalAssignment],
			map[identityCenterPrincipalAssignmentIndex]func(*identitycenterv1.PrincipalAssignment) string{
				identityCenterPrincipalAssignmentNameIndex: func(r *identitycenterv1.PrincipalAssignment) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*identitycenterv1.PrincipalAssignment, error) {
			var pageToken pagination.PageRequestToken
			var resources []*identitycenterv1.PrincipalAssignment
			for {
				resourcesPage, nextPage, err := upstream.ListPrincipalAssignments(ctx, 0, &pageToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				resources = append(resources, resourcesPage...)

				if nextPage == "" {
					break
				}
				pageToken.Update(nextPage)
			}
			return resources, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *identitycenterv1.PrincipalAssignment {
			return &identitycenterv1.PrincipalAssignment{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

func (c *Cache) GetPrincipalAssignment(ctx context.Context, id services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetPrincipalAssignment")
	defer span.End()

	getter := genericGetter[*identitycenterv1.PrincipalAssignment, identityCenterPrincipalAssignmentIndex]{
		cache:      c,
		collection: c.collections.identityCenterPrincipalAssignments,
		index:      identityCenterPrincipalAssignmentNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*identitycenterv1.PrincipalAssignment, error) {
			out, err := c.Config.IdentityCenter.GetPrincipalAssignment(ctx, services.PrincipalAssignmentID(s))
			return out, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, string(id))
	return out, trace.Wrap(err)
}

func (c *Cache) ListPrincipalAssignments(ctx context.Context, pageSize int, req *pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListPrincipalAssignments")
	defer span.End()

	lister := genericLister[*identitycenterv1.PrincipalAssignment, identityCenterPrincipalAssignmentIndex]{
		cache:      c,
		collection: c.collections.identityCenterPrincipalAssignments,
		index:      identityCenterPrincipalAssignmentNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, s string) ([]*identitycenterv1.PrincipalAssignment, string, error) {
			out, next, err := c.Config.IdentityCenter.ListPrincipalAssignments(ctx, pageSize, req)
			return out, string(next), trace.Wrap(err)
		},
		nextToken: func(t *identitycenterv1.PrincipalAssignment) string {
			return t.GetMetadata().GetName()
		},
	}

	nextToken, err := req.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out, next, err := lister.list(ctx, pageSize, nextToken)
	return out, pagination.NextPageToken(next), trace.Wrap(err)
}
