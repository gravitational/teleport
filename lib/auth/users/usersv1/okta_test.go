/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package usersv1_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

type builtinRoleAuthorizer struct{}

func (a builtinRoleAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	userI, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if role, ok := userI.(authz.BuiltinRole); ok {
		return authz.ContextForBuiltinRole(role, nil)
	}
	return nil, trace.AccessDenied("nope")
}

func newOktaUser(t *testing.T) types.User {
	t.Helper()
	user, err := types.NewUser(uuid.NewString())
	require.NoError(t, err)

	user.SetOrigin(types.OriginOkta)

	return user
}

// TestOktaCRUD() asserts that user operations involving Okta-origin
// users obey the following rules:
//
// 1. Only the Teleport Okta service may create an Okta-origin user.
// 2. Only the Teleport Okta service may modify an Okta-origin user.
// 3. Anyone with User RW can delete an Okta-origin user.
func TestOktaCRUD(t *testing.T) {
	env, err := newTestEnv(withAuthorizer(builtinRoleAuthorizer{}))
	require.NoError(t, err)

	oktaCtx := authz.ContextWithUser(
		context.Background(),
		authz.BuiltinRole{
			Role:     types.RoleOkta,
			Username: string(types.RoleOkta),
		})

	adminCtx := authz.ContextWithUser(
		context.Background(),
		authz.BuiltinRole{
			Role:     types.RoleAdmin,
			Username: string(types.RoleAdmin),
		})

	t.Run("Create", func(t *testing.T) {
		t.Run("okta service creating okta users is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			_, err := env.CreateUser(oktaCtx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
			require.NoError(t, err)
		})

		t.Run("okta service creating a non-okta user in an error", func(t *testing.T) {
			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)

			_, err = env.CreateUser(oktaCtx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service creating an okta user is an error", func(t *testing.T) {
			user := newOktaUser(t)

			_, err = env.CreateUser(adminCtx,
				&userspb.CreateUserRequest{User: user.(*types.UserV2)})
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("update", func(t *testing.T) {
		t.Run("okta service updating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and attempt to
			// update the backend...
			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation succeeds
			require.NoError(t, err)
		})

		t.Run("okta service updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and attempt to
			// update the backend...
			user.SetOrigin(types.OriginOkta)
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "access denied"
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
			require.Contains(t, err.Error(), "update")
		})

		t.Run("okta service removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record to remove the
			// Origin label, and attempt to update the backend...
			user.SetOrigin(types.OriginDynamic)
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "bad parameter"
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// When I (as the Okta service) try to update that user...
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that an error is returned rather than having the whole
			// system crash
			require.Error(t, err)
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record to remove the
			// Origin label, and attempt - as a non-okta service - to update the
			// backend as the...
			user.SetOrigin(types.OriginDynamic)
			_, err = env.UpdateUser(adminCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "bad parameter"
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and try - as a
			// non-okta service - to update the backend record...
			user.AddRole(teleport.PresetAccessRoleName)
			_, err = env.UpdateUser(adminCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "bad parameter"
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("upsert", func(t *testing.T) {
		t.Run("okta service creating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user NOT in the Teleport user DB...
			user := newOktaUser(t)

			// When I (as the Okta service) try to Upsert that user...
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to succeed
			require.NoError(t, err)
		})

		t.Run("okta service creating non-okta user is an error", func(t *testing.T) {
			// Given a non-okta user *not* already in the Teleport user backend...
			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)

			// When I (as the Okta service) try to Upsert that user...
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to fail with Bad Parameter
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service updating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user already in the Teleport user DB...
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and try, as the
			// Okta service, to upsert the changes into the backend...
			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to succeed
			require.NoError(t, err)
		})

		t.Run("okta service updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user already in the Teleport user DB...
			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and try, as the
			// Okta service, to upsert the changes into the backend...
			user.SetOrigin(types.OriginOkta)
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to fail with access denied
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta user already in the Teleport user DB...
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When remove the Okta origin label from the local copy of the user
			// record and then try, as the Okta service, to upsert the changes
			// into the backend...
			user.SetOrigin(types.OriginDynamic)
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to fail
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service creating okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)

			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record to remove the
			// Origin label, and attempt - as a non-okta service - to update the
			// backend as the...
			user.SetOrigin(types.OriginDynamic)
			_, err = env.UpsertUser(adminCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "bad parameter"
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and try - as a
			// non-okta service - to update the backend record...
			traits := map[string][]string{
				"foo": {"bar"},
			}
			user.SetTraits(traits)
			_, err = env.UpsertUser(adminCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "bad parameter"
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Run("okta service deleting Okta user is allowed", func(t *testing.T) {
			// Given an existing okta user already in the Teleport user DB...
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I (as the Okta service) try to delete the user...
			_, err = env.DeleteUser(
				oktaCtx,
				&userspb.DeleteUserRequest{Name: user.GetName()})

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect that the user has been removed from the cache/backend
			_, err = env.Service.GetUser(
				oktaCtx,
				&userspb.GetUserRequest{
					Name:        user.GetName(),
					WithSecrets: false,
				})
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err), "Expected not found, got %s", err.Error())
		})

		t.Run("okta service deleting non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user already in the Teleport user DB...
			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I (as the Okta service) try to delete the user...
			_, err = env.DeleteUser(
				oktaCtx,
				&userspb.DeleteUserRequest{Name: user.GetName()})

			// Expect the operation to fail with "access denied"
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
			require.Contains(t, err.Error(), "delete")

			// Expect that the user still exists in the cache/backend
			_, err = env.Service.GetUser(
				oktaCtx,
				&userspb.GetUserRequest{
					Name:        user.GetName(),
					WithSecrets: false,
				})
			require.NoError(t, err)
		})

		t.Run("non-okta service deleting okta user is allowed", func(t *testing.T) {
			// Given an existing okta user already in the Teleport user DB...
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I (as the Okta service) try to delete the user...
			_, err = env.DeleteUser(
				adminCtx,
				&userspb.DeleteUserRequest{Name: user.GetName()})

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect that the user has been removed from the cache/backend
			_, err = env.Service.GetUser(
				oktaCtx,
				&userspb.GetUserRequest{
					Name:        user.GetName(),
					WithSecrets: false,
				})
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err), "Expected not found, got %s", err.Error())
		})
	})
}
