// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usersv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

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
	user, err := types.NewUser(t.Name())
	require.NoError(t, err)

	m := user.GetMetadata()
	m.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
	user.SetMetadata(m)

	return user
}

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
		t.Run("creating Okta users is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			_, err := env.CreateUser(oktaCtx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
			require.NoError(t, err)
		})

		t.Run("creating non-Okta users in an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
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
		t.Run("updating Okta user is allowed", func(t *testing.T) {
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

		t.Run("updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and attempt to
			// update the backend...
			m := user.GetMetadata()
			m.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
			user.SetMetadata(m)
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "access denied"
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record to remove the
			// Origin label, and attempt to update the backend...
			m := user.GetMetadata()
			m.Labels[types.OriginLabel] = types.OriginDynamic
			user.SetMetadata(m)
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that the operation fails with "bad parameter"
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// When I (as the Okta service) try to update that user...
			_, err = env.UpdateUser(oktaCtx,
				&userspb.UpdateUserRequest{User: user.(*types.UserV2)})

			// Expect that an error is returned rather than having the whole
			// system crash
			require.Error(t, err)
		})
	})

	t.Run("upsert", func(t *testing.T) {
		t.Run("creating non-Okta user is an error", func(t *testing.T) {
			// Given a non-okta user *not* already in the Teleport user backend...
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			// When I (as the Okta service) try to Upsert that user...
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to fail with Bad Parameter
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("creating Okta user is allowed", func(t *testing.T) {
			// Given an existing okta user NOT in the Teleport user DB...
			user := newOktaUser(t)

			// When I (as the Okta service) try to Upsert that user...
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to succeed
			require.NoError(t, err)
		})

		t.Run("updating Okta user is allowed", func(t *testing.T) {
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

		t.Run("updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user already in the Teleport user DB...
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When I modify the local copy of the user record and try, as the
			// Okta service, to upsert the changes into the backend...
			m := user.GetMetadata()
			m.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
			user.SetMetadata(m)
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to fail with access denied
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta user already in the Teleport user DB...
			user := newOktaUser(t)
			user, err = env.backend.CreateUser(context.Background(), user)
			require.NoError(t, err)

			// When remove the Okta origin label from the local copy of the user
			// record and then try, as the Okta service, to upsert the changes
			// into the backend...
			m := user.GetMetadata()
			m.Labels[types.OriginLabel] = types.OriginDynamic
			user.SetMetadata(m)
			user.SetMetadata(m)
			_, err = env.UpsertUser(
				oktaCtx,
				&userspb.UpsertUserRequest{User: user.(*types.UserV2)})

			// Expect the operation to fail
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Run("deleting Okta user is allowed", func(t *testing.T) {
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
			_, err = env.cache.GetUser(context.Background(), user.GetName(), false)
			require.True(t, trace.IsNotFound(err), "Expected not found, got %s", err.Error())
		})

		t.Run("deleting non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user already in the Teleport user DB...
			user, err := types.NewUser(t.Name())
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

			// Expect that the user still exists in the cache/backend
			_, err = env.cache.GetUser(context.Background(), user.GetName(), false)
			require.NoError(t, err)
		})
	})
}
