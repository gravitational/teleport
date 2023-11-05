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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

func newOktaUser(t *testing.T) types.User {
	t.Helper()
	user, err := types.NewUser(t.Name())
	require.NoError(t, err)

	m := user.GetMetadata()
	m.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
	user.SetMetadata(m)

	return user
}

// TODO(tcsc): DELETE IN 16.0.0
func TestOktaServiceUserCRUD(t *testing.T) {
	// Given an RBAC-checking `ServerWithRoles` configured with the built-in
	// Okta Role...
	ctx := context.Background()

	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, TestBuiltin(types.RoleOkta).I))
	require.NoError(t, err)

	authWithOktaRole := &ServerWithRoles{
		authServer: srv.AuthServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}

	t.Run("create", func(t *testing.T) {
		t.Run("creating Okta users is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			_, err = authWithOktaRole.CreateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("creating non-Okta users in an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = authWithOktaRole.CreateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("non-okta service creating an okta user is an error", func(t *testing.T) {
			authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, TestBuiltin(types.RoleAdmin).I))
			require.NoError(t, err)

			authWithBotRole := &ServerWithRoles{
				authServer: srv.AuthServer,
				alog:       srv.AuditLog,
				context:    *authContext,
			}
			defer authWithBotRole.Close()

			user := newOktaUser(t)

			_, err = authWithBotRole.CreateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("update", func(t *testing.T) {
		t.Run("updating Okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			m := user.GetMetadata()
			m.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
			user.SetMetadata(m)

			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			m := user.GetMetadata()
			m.Labels[types.OriginLabel] = types.OriginDynamic
			user.SetMetadata(m)

			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// when I try to update that user, an error is returned rather than
			// having the whole system crash
			_, err = authWithOktaRole.UpdateUser(ctx, user)
			require.Error(t, err)
		})
	})

	t.Run("upsert", func(t *testing.T) {
		t.Run("creating non-Okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("creating Okta user is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			_, err = authWithOktaRole.CreateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("updating Okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			m := user.GetMetadata()
			m.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
			user.SetMetadata(m)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			m := user.GetMetadata()
			m.Labels[types.OriginLabel] = types.OriginDynamic
			user.SetMetadata(m)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})
	})

	t.Run("compare and swap", func(t *testing.T) {
		t.Run("updating Okta user is allowed", func(t *testing.T) {
			// Given an existing okta existing
			existing := newOktaUser(t)
			existing, err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, existing.GetName(), false)
			require.NoError(t, err)
			modified.SetTraits(map[string][]string{"foo": {"bar", "baz"}})

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.NoError(t, err)
		})

		t.Run("updating non-Okta user is an error", func(t *testing.T) {
			// Given an existing non-okta existing
			existing, err := types.NewUser(t.Name())
			require.NoError(t, err)
			existing, err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, existing.GetName(), false)
			require.NoError(t, err)
			metadata := modified.GetMetadata()
			metadata.Labels = map[string]string{types.OriginLabel: types.OriginOkta}
			modified.SetMetadata(metadata)

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})

		t.Run("removing Okta origin is an error", func(t *testing.T) {
			// Given an existing okta existing
			existing := newOktaUser(t)
			existing, err = srv.AuthServer.CreateUser(ctx, existing)
			require.NoError(t, err)

			modified, err := srv.AuthServer.GetUser(ctx, existing.GetName(), false)
			require.NoError(t, err)
			metadata := modified.GetMetadata()
			metadata.Labels = map[string]string{types.OriginLabel: types.OriginDynamic}
			modified.SetMetadata(metadata)

			err = authWithOktaRole.CompareAndSwapUser(ctx, modified, existing)
			require.Error(t, err)
			require.Truef(t, trace.IsBadParameter(err), "Expected bad parameter, got %T: %s", err, err.Error())
		})

		t.Run("updating a non-existent user is an error", func(t *testing.T) {
			// Given an okta user not present in the user DB
			user := newOktaUser(t)

			// when I try to update that user, an error is returned rather than
			// having the whole system crash
			err := authWithOktaRole.CompareAndSwapUser(ctx, user, user)
			require.Error(t, err)
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Run("deleting Okta user is allowed", func(t *testing.T) {
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			err = authWithOktaRole.DeleteUser(ctx, user.GetName())
			require.NoError(t, err)

			_, err = srv.AuthServer.GetUser(ctx, user.GetName(), false)
			require.True(t, trace.IsNotFound(err), "Expected not found, got %s", err.Error())
		})

		t.Run("deleting non-Okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			err = authWithOktaRole.DeleteUser(ctx, user.GetName())
			require.Error(t, err)
			require.Truef(t, trace.IsAccessDenied(err), "Expected access denied, got %T: %s", err, err.Error())
		})
	})
}
