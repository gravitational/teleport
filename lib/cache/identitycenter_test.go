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
	"testing"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
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

// TestIdentityCenterAccount asserts that an Identity Center Account can be cached
func TestIdentityCenterAccount(t *testing.T) {
	t.Parallel()

	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs[*identitycenterv1.Account]{
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
		list: fixturePack.identityCenter.ListIdentityCenterAccounts,
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteIdentityCenterAccount(
				ctx, services.IdentityCenterAccountID(id)))
		},
		deleteAll: fixturePack.identityCenter.DeleteAllIdentityCenterAccounts,
		cacheList: fixturePack.cache.ListIdentityCenterAccounts,
		cacheGet:  fixturePack.cache.GetIdentityCenterAccount,
	}, withSkipPaginationTest())
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

// TestIdentityCenterPrincipalAssignment asserts that an Identity Center PrincipalAssignment can be cached
func TestIdentityCenterPrincipalAssignment(t *testing.T) {
	t.Parallel()
	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs[*identitycenterv1.PrincipalAssignment]{
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
		list: fixturePack.identityCenter.ListPrincipalAssignments,
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeletePrincipalAssignment(ctx, services.PrincipalAssignmentID(id)))
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAllPrincipalAssignments(ctx))
		},
		cacheList: fixturePack.cache.ListPrincipalAssignments,
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.PrincipalAssignment, error) {
			r, err := fixturePack.cache.GetPrincipalAssignment(ctx, services.PrincipalAssignmentID(id))
			return r, trace.Wrap(err)
		},
	}, withSkipPaginationTest())
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
	t.Parallel()
	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs[*identitycenterv1.AccountAssignment]{
		newResource: func(s string) (*identitycenterv1.AccountAssignment, error) {
			return newIdentityCenterAccountAssignment(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.AccountAssignment) error {
			_, err := fixturePack.identityCenter.CreateIdentityCenterAccountAssignment(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.AccountAssignment) error {
			_, err := fixturePack.identityCenter.UpdateIdentityCenterAccountAssignment(ctx, item)
			return trace.Wrap(err)
		},
		list: fixturePack.identityCenter.ListIdentityCenterAccountAssignments,
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAccountAssignment(ctx, services.IdentityCenterAccountAssignmentID(id)))
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAllAccountAssignments(ctx))
		},
		cacheList: fixturePack.cache.ListIdentityCenterAccountAssignments,
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.AccountAssignment, error) {
			r, err := fixturePack.cache.GetAccountAssignment(ctx, services.IdentityCenterAccountAssignmentID(id))
			return r.AccountAssignment, trace.Wrap(err)
		},
	}, withSkipPaginationTest())
}
