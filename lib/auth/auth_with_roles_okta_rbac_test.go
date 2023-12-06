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

	t.Run("update", func(t *testing.T) {
		t.Run("okta service updating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err := srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as the Okta service) modify the user and attempt to update the backend record...
			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			// Expect the operation to succeed
			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("okta service updating non-okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as the Okta service) attempt modify that user
			user.SetOrigin(types.OriginOkta)
			_, err = authWithOktaRole.UpdateUser(ctx, user)

			// Expect the attempt to fail
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as the Okta service) attempt reset the user origin
			user.SetOrigin(types.OriginDynamic)

			// Expect the attempt to fail
			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// when I try to update that user, an error is returned rather than
			// having the whole system crash
			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as a non-Okta service) attempt reset the user origin
			user.SetOrigin(types.OriginDynamic)

			// Expect the attempt to fail
			_, err = authWithAdminRole.UpdateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			// When I (as a non-Okta service) attempt modify that user
			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})
			_, err = authWithAdminRole.UpdateUser(ctx, user)

			// Expect the attempt to fail
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter got %T: %s", err, err.Error())
		})
	})

	t.Run("upsert", func(t *testing.T) {
		t.Run("okta service creating non-okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("okta service updating okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("okta service updating non-okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginOkta)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing the okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginDynamic)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service creating okta user is an error", func(t *testing.T) {
			user := newOktaUser(t)
			_, err = authWithAdminRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginDynamic)

			_, err = authWithAdminRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.AddRole(teleport.PresetAccessRoleName)

			_, err = authWithAdminRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("compare and swap", func(t *testing.T) {
		t.Run("okta service updating Okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			existing := newOktaUser(t)
			existing, err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, existing.GetName(), false)
			require.NoError(t, err)
			modified.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.NoError(t, err)
		})

		t.Run("okta service updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta existing
			existing, err := types.NewUser(t.Name())
			require.NoError(t, err)
			existing, err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, existing.GetName(), false)
			require.NoError(t, err)
			modified.SetOrigin(types.OriginOkta)

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("okta service removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta existing
			existing := newOktaUser(t)
			existing, err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, existing.GetName(), false)
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
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			original, err := srv.AuthServer.GetUser(ctx, user.GetName(), false)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, user.GetName(), false)
			require.NoError(t, err)
			modified.SetOrigin(types.OriginDynamic)

			err = authWithAdminRole.CompareAndSwapUser(ctx, modified, original)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			original, err := srv.AuthServer.GetUser(ctx, user.GetName(), false)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, user.GetName(), false)
			require.NoError(t, err)
			modified.AddRole(teleport.PresetAccessRoleName)

			err = authWithAdminRole.CompareAndSwapUser(ctx, modified, original)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
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
		existing, err = srv.AuthServer.CreateUser(ctx, existing)
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
		existing, err = srv.AuthServer.CreateUser(ctx, existing)
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
