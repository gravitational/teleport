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
	"log/slog"
	"os"
	"testing"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func newIdentityCenterAccount(id string) *identitycenterv1.Account {
	return &identitycenterv1.Account{
		Kind:    types.KindIdentityCenterAccount,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &identitycenterv1.AccountSpec{
			Id:  id,
			Arn: "arn:aws:sso:::permissionSet/ssoins-722326ecc902a06a/" + id,
		},
		Status: &identitycenterv1.AccountStatus{},
	}
}

// TestIdentityCenterAccount asserts that an Identoty Ceneter Account can be cached
func TestIdentityCenterAccount(t *testing.T) {
	slog.SetDefault(
		slog.New(logutils.NewSlogTextHandler(
			os.Stderr, logutils.SlogTextHandlerConfig{Level: slog.LevelDebug})))

	t.Parallel()

	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	collect := func(ctx context.Context, src identityCenterAccountGetter) ([]*identitycenterv1.Account, error) {
		var result []*identitycenterv1.Account
		var pageToken string
		for {
			page, nextPage, err := src.ListIdentityCenterAccounts(ctx, 0, pageToken)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result = append(result, page...)

			pageToken = nextPage
			if nextPage == "" {
				break
			}
		}
		return result, nil
	}

	testResources153(t, fixturePack, testFuncs153[*identitycenterv1.Account]{
		newResource: func(s string) (*identitycenterv1.Account, error) {
			return newIdentityCenterAccount(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.Account) error {
			_, err := fixturePack.identityCenter.CreateIdentityCenterAccount(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.Account) error {
			_, err := fixturePack.identityCenter.UpdateIdentityCenterAccount(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*identitycenterv1.Account, error) {
			return collect(ctx, fixturePack.identityCenter)
		},
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteIdentityCenterAccount(ctx, id))
		},
		deleteAll: func(ctx context.Context) error {
			err := fixturePack.identityCenter.DeleteAllIdentityCenterAccounts(ctx, &identitycenterv1.DeleteAllIdentityCenterAccountsRequest{})
			return trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*identitycenterv1.Account, error) {
			return collect(ctx, fixturePack.cache.identityCenterCache)
		},
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.Account, error) {
			r, err := fixturePack.cache.identityCenterCache.GetIdentityCenterAccount(ctx, id)
			return r, trace.Wrap(err)
		},
	})
}

func newIdentityCenterPrincipalAssignment(id string) *identitycenterv1.PrincipalAssignment {
	return &identitycenterv1.PrincipalAssignment{
		Kind:    types.KindIdentityCenterPrincipalAssignment,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &identitycenterv1.PrincipalAssignmentSpec{
			PrincipalType: identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER,
			PrincipalId:   id,
			ExternalId:    "ext_" + id,
		},
		Status: &identitycenterv1.PrincipalAssignmentStatus{
			ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED,
		},
	}
}

// TestIdentityCenterPrincpialAssignment asserts that an Identity Center PrincipalAssignment can be cached
func TestIdentityCenterPrincipalAssignment(t *testing.T) {
	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	collect := func(ctx context.Context, src identityCenterPrincipalAssignmentGetter) ([]*identitycenterv1.PrincipalAssignment, error) {
		var result []*identitycenterv1.PrincipalAssignment
		var pageToken string
		for {
			page, nextPage, err := src.ListPrincipalAssignments(ctx, 0, pageToken)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result = append(result, page...)

			pageToken = nextPage
			if nextPage == "" {
				break
			}
		}
		return result, nil
	}

	testResources153(t, fixturePack, testFuncs153[*identitycenterv1.PrincipalAssignment]{
		newResource: func(s string) (*identitycenterv1.PrincipalAssignment, error) {
			return newIdentityCenterPrincipalAssignment(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.PrincipalAssignment) error {
			_, err := fixturePack.identityCenter.CreatePrincipalAssignment(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.PrincipalAssignment) error {
			_, err := fixturePack.identityCenter.UpdatePrincipalAssignment(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*identitycenterv1.PrincipalAssignment, error) {
			return collect(ctx, fixturePack.identityCenter)
		},
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeletePrincipalAssignment(ctx, id))
		},
		deleteAll: func(ctx context.Context) error {
			err := fixturePack.identityCenter.DeleteAllPrincipalAssignments(ctx, &identitycenterv1.DeleteAllPrincipalAssignmentsRequest{})
			return trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*identitycenterv1.PrincipalAssignment, error) {
			return collect(ctx, fixturePack.cache.identityCenterCache)
		},
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.PrincipalAssignment, error) {
			r, err := fixturePack.cache.identityCenterCache.GetPrincipalAssignment(ctx, id)
			return r, trace.Wrap(err)
		},
	})
}

func newIdentityCenterAccountAssignment(id string) *identitycenterv1.AccountAssignment {
	return &identitycenterv1.AccountAssignment{
		Kind:    types.KindIdentityCenterAccountAssignment,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &identitycenterv1.AccountAssignmentSpec{
			Display:       "account " + id,
			PermissionSet: &identitycenterv1.PermissionSetInfo{},
			AccountName:   id,
			AccountId:     id,
		},
	}
}

// TestIdentityCenterAccountAssignment asserts that an Identity Center
// AccountAssignment can be cached
func TestIdentityCenterAccountAssignment(t *testing.T) {
	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	collect := func(ctx context.Context, src identityCenterAccountAssignmentGetter) ([]*identitycenterv1.AccountAssignment, error) {
		var result []*identitycenterv1.AccountAssignment
		var pageToken string
		for {
			page, nextPage, err := src.ListAccountAssignments(ctx, 0, pageToken)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result = append(result, page...)

			pageToken = nextPage
			if nextPage == "" {
				break
			}
		}
		return result, nil
	}

	testResources153(t, fixturePack, testFuncs153[*identitycenterv1.AccountAssignment]{
		newResource: func(s string) (*identitycenterv1.AccountAssignment, error) {
			return newIdentityCenterAccountAssignment(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.AccountAssignment) error {
			_, err := fixturePack.identityCenter.CreateAccountAssignment(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.AccountAssignment) error {
			_, err := fixturePack.identityCenter.UpdateAccountAssignment(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*identitycenterv1.AccountAssignment, error) {
			return collect(ctx, fixturePack.identityCenter)
		},
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAccountAssignment(ctx, id))
		},
		deleteAll: func(ctx context.Context) error {
			err := fixturePack.identityCenter.DeleteAllAccountAssignments(ctx, &identitycenterv1.DeleteAllAccountAssignmentsRequest{})
			return trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*identitycenterv1.AccountAssignment, error) {
			return collect(ctx, fixturePack.cache.identityCenterCache)
		},
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.AccountAssignment, error) {
			r, err := fixturePack.cache.identityCenterCache.GetAccountAssignment(ctx, id)
			return r, trace.Wrap(err)
		},
	})
}
