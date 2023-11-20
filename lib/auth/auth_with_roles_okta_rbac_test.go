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

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

func newOktaUser(t *testing.T) types.User {
	t.Helper()
	user, err := types.NewUser(t.Name())
	require.NoError(t, err)

	user.SetOrigin(types.OriginOkta)

	return user
}

// newTestServerWithRoles creates a self-cleaning `ServerWithRoles`, configured
// with a given
func newTestServerWithRoles(t *testing.T, srv *TestAuthServer, role types.SystemRole) *ServerWithRoles {

	authzContext := authz.ContextWithUser(context.Background(), TestBuiltin(role).I)
	ctxIdentity, err := srv.Authorizer.Authorize(authzContext)
	require.NoError(t, err)

	authWithRole := &ServerWithRoles{
		authServer: srv.AuthServer,
		alog:       srv.AuditLog,
		context:    *ctxIdentity,
	}

	t.Cleanup(func() { authWithRole.Close() })

	return authWithRole
}

// TestOktaServiceUserCRUD() asserts that user operations involving Okta-origin
// users obey the following rules:
//
// 1. Only the Teleport Okta service may create an Okta-origin user.
// 2. Only the Teleport Okta service may modify an Okta-origin user.
// 3. Anyone with User RW can delete an Okta-origin user.
//
// TODO(tcsc): DELETE IN 16.0.0 (or when user management is excised from ServerWithRoles)
func TestOktaServiceUserCRUD(t *testing.T) {
	// Given an RBAC-checking `ServerWithRoles` configured with the built-in
	// Okta Role...
	ctx := context.Background()

	// Given an auth server...
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	// And an RBAC-checking `ServerWithRoles` facade configured with the
	// built-in Okta Role...
	authWithOktaRole := newTestServerWithRoles(t, srv, types.RoleOkta)

	// And another RBAC-checking `ServerWithRoles` facade configured with the
	// something other than the built-in Okta Role...
	authWithAdminRole := newTestServerWithRoles(t, srv, types.RoleAdmin)

	t.Run("create", func(t *testing.T) {
		t.Run("okta service creating okta users is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			err := authWithOktaRole.CreateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("okta service creating non-okta users in an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			err = authWithOktaRole.CreateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service creating an okta user is an error", func(t *testing.T) {
			user := newOktaUser(t)

			err := authWithAdminRole.CreateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("update", func(t *testing.T) {
		t.Run("okta service updating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err := srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as the Okta service) modify the user and attempt to update the backend record...
			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			// Expect the operation to succeed
			err = authWithOktaRole.UpdateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("okta service updating non-okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as the Okta service) attempt modify that user
			user.SetOrigin(types.OriginOkta)
			err = authWithOktaRole.UpdateUser(ctx, user)

			// Expect the attempt to fail
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as the Okta service) attempt reset the user origin
			user.SetOrigin(types.OriginDynamic)

			// Expect the attempt to fail
			err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// when I try to update that user, an error is returned rather than
			// having the whole system crash
			err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as a non-Okta service) attempt reset the user origin
			user.SetOrigin(types.OriginDynamic)

			// Expect the attempt to fail
			err := authWithAdminRole.UpdateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err := srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as a non-Okta service) attempt modify that user
			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})
			err = authWithAdminRole.UpdateUser(ctx, user)

			// Expect the attempt to fail
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter got %T: %s", err, err.Error())
		})
	})

	t.Run("upsert", func(t *testing.T) {
		t.Run("okta service creating non-okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			err = authWithOktaRole.UpsertUser(user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service creating okta user is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			err = authWithOktaRole.CreateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("okta service updating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			err = authWithOktaRole.UpsertUser(user)
			require.NoError(t, err)
		})

		t.Run("okta service updating non-okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginOkta)

			err = authWithOktaRole.UpsertUser(user)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing the okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginDynamic)

			err = authWithOktaRole.UpsertUser(user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service creating okta user is an error", func(t *testing.T) {
			user := newOktaUser(t)
			err := authWithAdminRole.UpsertUser(user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginDynamic)

			err = authWithAdminRole.UpsertUser(user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.AddRole(teleport.PresetAccessRoleName)

			err = authWithAdminRole.UpsertUser(user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("compare and swap", func(t *testing.T) {
		t.Run("okta service updating Okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			existing := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(existing.GetName(), false)
			require.NoError(t, err)
			modified.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.NoError(t, err)
		})

		t.Run("okta service updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta existing
			existing, err := types.NewUser(t.Name())
			require.NoError(t, err)
			err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(existing.GetName(), false)
			require.NoError(t, err)
			modified.SetOrigin(types.OriginOkta)

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta existing
			existing := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(existing.GetName(), false)
			require.NoError(t, err)
			modified.SetOrigin(types.OriginDynamic)

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// when I try to update that user, an error is returned rather than
			// having the whole system crash
			err := authWithOktaRole.CompareAndSwapUser(ctx, user, user)
			require.Error(t, err)
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			original, err := srv.AuthServer.GetUser(user.GetName(), false)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(user.GetName(), false)
			require.NoError(t, err)
			modified.SetOrigin(types.OriginDynamic)

			err = authWithAdminRole.CompareAndSwapUser(ctx, modified, original)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			err := srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			original, err := srv.AuthServer.GetUser(user.GetName(), false)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(user.GetName(), false)
			require.NoError(t, err)
			modified.AddRole(teleport.PresetAccessRoleName)

			err = authWithAdminRole.CompareAndSwapUser(ctx, modified, original)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Run("okta service deleting Okta user is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			err = authWithOktaRole.DeleteUser(ctx, user.GetName())
			require.NoError(t, err)

			_, err = srv.AuthServer.GetUser(user.GetName(), false)
			require.True(t, trace.IsNotFound(err), "Expected not found, got %s", err.Error())
		})

		t.Run("okta service deleting non-Okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			err = authWithOktaRole.DeleteUser(ctx, user.GetName())
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service deleting Okta user is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			err = authWithAdminRole.DeleteUser(ctx, user.GetName())
			require.NoError(t, err)

			_, err = srv.AuthServer.GetUser(user.GetName(), false)
			require.True(t, trace.IsNotFound(err), "Expected not found, got %s", err.Error())
		})
	})
}

func TestOktaMayNotResetPasswords(t *testing.T) {
	ctx := context.Background()

	// Given an auth server...
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	// And an RBAC-checking `ServerWithRoles` facade configured with the
	// built-in Okta Role...
	authWithOktaRole := newTestServerWithRoles(t, srv, types.RoleOkta)

	t.Run("okta user", func(t *testing.T) {
		// Given an existing okta in the user DB
		existing := newOktaUser(t)
		err = srv.AuthServer.CreateUser(ctx, existing)
		require.NoError(t, err)

		_, err = authWithOktaRole.CreateResetPasswordToken(ctx,
			CreateUserTokenRequest{Name: existing.GetName()})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
	})

	t.Run("non-okta user", func(t *testing.T) {
		// Given an existing non-okta existing
		existing, err := types.NewUser(t.Name())
		require.NoError(t, err)
		err = srv.AuthServer.CreateUser(ctx, existing)
		require.NoError(t, err)

		_, err = authWithOktaRole.CreateResetPasswordToken(ctx,
			CreateUserTokenRequest{Name: existing.GetName()})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
	})

	t.Run("resetting non-existent user must not leak info", func(t *testing.T) {
		// Given a request to reset the password for a non-existent
		// user, when I try to reset the token
		_, err = authWithOktaRole.CreateResetPasswordToken(ctx,
			CreateUserTokenRequest{Name: t.Name()})

		// Expect the operation to fail with "access denied" rather
		// than "not found", so as not to leak the existence of the
		// user with different error codes
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
	})
}

func TestOktaMayNotCreateBotUser(t *testing.T) {
	ctx := context.Background()

	// Given an auth server...
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	// And an RBAC-checking `ServerWithRoles` facade configured with the
	// built-in Okta Role...
	authWithOktaRole := newTestServerWithRoles(t, srv, types.RoleOkta)

	// When I attempt to create a Bot user
	_, err = authWithOktaRole.CreateBot(ctx, &proto.CreateBotRequest{
		Name:  t.Name(),
		Roles: []string{string(types.RoleDiscovery)},
	})

	// The attempt should fail with access denied
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
}
