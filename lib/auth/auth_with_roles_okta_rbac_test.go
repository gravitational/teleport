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
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
)

func newUserWithOrigin(t *testing.T, origin string) types.User {
	t.Helper()
	user, err := types.NewUser(t.Name())
	require.NoError(t, err)

	if origin != "" {
		user.SetOrigin(origin)
	}

	return user
}

func newOktaUser(t *testing.T) types.User {
	return newUserWithOrigin(t, types.OriginOkta)
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
			requireAccessDenied(t, err)
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
			requireBadParameter(t, err)
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
			requireBadParameter(t, err)
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
			requireBadParameter(t, err)
		})
	})

	t.Run("upsert", func(t *testing.T) {
		t.Run("okta service creating non-okta user is an error", func(t *testing.T) {
			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			requireBadParameter(t, err)
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
			requireAccessDenied(t, err)
		})

		t.Run("okta service removing the okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginDynamic)

			_, err = authWithOktaRole.UpsertUser(ctx, user)
			requireBadParameter(t, err)
		})

		t.Run("non-okta service creating okta user is an error", func(t *testing.T) {
			user := newOktaUser(t)
			_, err = authWithAdminRole.UpsertUser(ctx, user)
			requireBadParameter(t, err)
		})

		t.Run("non-okta service removing okta origin is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.SetOrigin(types.OriginDynamic)

			_, err = authWithAdminRole.UpsertUser(ctx, user)
			requireBadParameter(t, err)
		})

		t.Run("non-okta service updating an okta user is an error", func(t *testing.T) {
			// Given an existing okta user
			user := newOktaUser(t)
			user, err = srv.AuthServer.CreateUser(ctx, user)
			require.NoError(t, err)

			user.AddRole(teleport.PresetAccessRoleName)

			_, err = authWithAdminRole.UpsertUser(ctx, user)
			requireBadParameter(t, err)
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
			requireAccessDenied(t, err)
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
			requireBadParameter(t, err)
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
			requireBadParameter(t, err)
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
			requireBadParameter(t, err)
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
			authclient.CreateUserTokenRequest{Name: existing.GetName()})
		requireAccessDenied(t, err)
	})

	t.Run("non-okta user", func(t *testing.T) {
		// Given an existing non-okta existing
		existing, err := types.NewUser(t.Name())
		require.NoError(t, err)
		existing, err = srv.AuthServer.CreateUser(ctx, existing)
		require.NoError(t, err)

		_, err = authWithOktaRole.CreateResetPasswordToken(ctx,
			authclient.CreateUserTokenRequest{Name: existing.GetName()})
		requireAccessDenied(t, err)
	})

	t.Run("resetting non-existent user must not leak info", func(t *testing.T) {
		// Given a request to reset the password for a non-existent
		// user, when I try to reset the token
		_, err = authWithOktaRole.CreateResetPasswordToken(ctx,
			authclient.CreateUserTokenRequest{Name: t.Name()})

		// Expect the operation to fail with "access denied" rather
		// than "not found", so as not to leak the existence of the
		// user with different error codes
		requireAccessDenied(t, err)
	})
}

func newTestLock(t *testing.T, target types.User, origin string) types.Lock {
	lockSpec := types.LockSpecV2{
		Target: types.LockTarget{
			User: target.GetName(),
		},
	}
	lock, err := types.NewLock(t.Name(), lockSpec)
	require.NoError(t, err)

	if origin != "" {
		lock.SetOrigin(origin)
	}
	return lock
}

func TestOktaServiceLockCRUD(t *testing.T) {
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

	newLockInCluster := func(innerT *testing.T, target types.User, origin string) types.Lock {
		lock := newTestLock(innerT, target, origin)
		require.NoError(innerT, srv.AuthServer.UpsertLock(context.Background(), lock))
		return lock
	}

	newUserInCluster := func(innerT *testing.T, origin string) types.User {
		user := newUserWithOrigin(innerT, origin)
		created, err := srv.AuthServer.CreateUser(context.Background(), user)
		require.NoError(innerT, err)
		return created
	}

	t.Run("upsert", func(t *testing.T) {
		t.Run("okta service locking an okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			targetUser := newUserInCluster(t, types.OriginOkta)

			// When I (as the Okta service) attempt to create a lock on the
			// target user, with the lock's origin set to Okta
			lock := newTestLock(t, targetUser, types.OriginOkta)
			err = authWithOktaRole.UpsertLock(ctx, lock)

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect that lock now exists
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
		})

		t.Run("okta service creating a lock without origin is an error", func(t *testing.T) {
			// Given an existing okta user
			targetUser := newUserInCluster(t, types.OriginOkta)

			// When I (as the Okta service) attempt to create a lock on the
			// target user, but do NOT set the lock's origin
			lock := newTestLock(t, targetUser, "")

			// Expect the operation to fail with bad parameter
			err = authWithOktaRole.UpsertLock(ctx, lock)
			requireBadParameter(t, err)

			// Expect no such lock to exist
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			requireNotFound(t, err)
		})

		t.Run("okta service locking a non-okta user is an error", func(t *testing.T) {
			// Given an existing non-okta user
			targetUser := newUserInCluster(t, "")

			// When I (as the Okta service) attempt to create lock on that
			// user...
			lock := newTestLock(t, targetUser, types.OriginOkta)

			// Expect the operation to fail with access denied
			err = authWithOktaRole.UpsertLock(ctx, lock)
			requireAccessDenied(t, err)

			// Expect no such lock to exist
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			requireNotFound(t, err)
		})

		t.Run("okta service locking anything other than a user is an error", func(t *testing.T) {
			// When I (as the Okta service) attempt to create lock on a random resource
			lockSpec := types.LockSpecV2{
				Target: types.LockTarget{
					Node: "banana",
				},
			}
			lock, err := types.NewLock(t.Name(), lockSpec)
			require.NoError(t, err)
			lock.SetOrigin(types.OriginOkta)

			// Expect the operation to fail with bad parameter
			err = authWithOktaRole.UpsertLock(ctx, lock)
			requireBadParameter(t, err)

			// Expect no such lock to exist
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			requireNotFound(t, err)
		})

		t.Run("non-okta service creating an okta lock is an error", func(t *testing.T) {
			// Given an existing okta user
			targetUser := newUserInCluster(t, types.OriginOkta)

			// When I (as something other than the Okta service) attempt to
			// create a lock on the target user, with the lock's origin set to
			// Okta...
			lock := newTestLock(t, targetUser, types.OriginOkta)
			err = authWithAdminRole.UpsertLock(ctx, lock)

			// Expect the operation to fail
			requireBadParameter(t, err)

			// Expect that no such lock exists
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			requireNotFound(t, err)
		})

		t.Run("non-okta service locking an okta user is allowed", func(t *testing.T) {
			// Given an existing okta user
			targetUser := newUserInCluster(t, types.OriginOkta)

			// When I, as someone other than the Okta service, attempt to create
			// a lock on the target Okta user...
			lock := newTestLock(t, targetUser, "")
			err = authWithAdminRole.UpsertLock(ctx, lock)

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect that lock now exists
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
		})
	})

	t.Run("update", func(t *testing.T) {
		t.Run("okta service updating okta lock is allowed", func(t *testing.T) {
			// Given an existing okta user with an existing okta-sourced lock
			targetUser := newUserInCluster(t, types.OriginOkta)
			lock := newLockInCluster(t, targetUser, types.OriginOkta)

			// When I attempt to update the lock
			lock.SetExpiry(time.Date(2233, time.March, 22, 00, 00, 00, 00, time.UTC))
			err = authWithOktaRole.UpsertLock(ctx, lock)

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect that the backend lock was updated
			updated, err := srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
			require.Equal(t, lock.Expiry(), updated.Expiry())
		})

		t.Run("okta service removing okta origin label is an error", func(t *testing.T) {
			// Given an existing okta user with an existing okta-sourced lock
			targetUser := newUserInCluster(t, types.OriginOkta)
			lock := newLockInCluster(t, targetUser, types.OriginOkta)

			// When I attempt to update the lock to change the Okta origin label
			lock.SetOrigin(types.OriginKubernetes)

			// Expect the operation to fail with bad parameter
			err = authWithOktaRole.UpsertLock(ctx, lock)
			requireBadParameter(t, err)

			// Expect that the backend lock was NOT updated
			backendLock, err := srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
			require.Equal(t, types.OriginOkta, backendLock.Origin())
		})

		t.Run("okta service changing lock to target non-okta user is an error", func(t *testing.T) {
			// Given an existing okta user with an existing okta-sourced lock
			targetUser := newUserInCluster(t, types.OriginOkta)
			lock := newLockInCluster(t, targetUser, types.OriginOkta)

			// And an un-locked non-okta user
			nonOktaUser := newUserWithOrigin(t, "")
			nonOktaUser.SetName(t.Name() + "non-okta")
			_, err = srv.AuthServer.CreateUser(ctx, nonOktaUser)
			require.NoError(t, err)

			// When the okta service attempts to update the lock to target the
			// non-okta user
			target := lock.Target()
			target.User = nonOktaUser.GetName()
			lock.SetTarget(target)
			err = authWithOktaRole.UpsertLock(ctx, lock)

			// Expect the operation to fail with access denied
			requireAccessDenied(t, err)

			// Expect that the backend lock was NOT updated
			backendLock, err := srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
			require.Equal(t, targetUser.GetName(), backendLock.Target().User)
		})

		t.Run("okta service editing non-okta lock is an error", func(t *testing.T) {
			// Given a non okta-origin lock in the cluster...
			targetUser := newUserInCluster(t, "")
			lock := newLockInCluster(t, targetUser, "")

			// When the Okta service attempts to edit that lock
			lock.SetExpiry(time.Date(1752, time.October, 14, 00, 00, 00, 00, time.UTC))
			lock.SetOrigin(types.OriginOkta)
			err = authWithOktaRole.UpsertLock(ctx, lock)

			// Expect the operation to fail with access denied
			requireAccessDenied(t, err)

			// Expect that the backend lock was NOT updated
			backendLock, err := srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
			require.Equal(t, time.Time{}, backendLock.Expiry())
		})

		t.Run("non-okta service updating okta lock is an error", func(t *testing.T) {
			// Given an existing okta user with an existing okta-sourced lock
			targetUser := newUserInCluster(t, types.OriginOkta)
			lock := newLockInCluster(t, targetUser, types.OriginOkta)

			// When I attempt to update the lock as a non-okta user
			lock.SetExpiry(time.Date(2233, time.March, 22, 00, 00, 00, 00, time.UTC))
			err = authWithAdminRole.UpsertLock(ctx, lock)

			// Expect the operation to fail
			requireBadParameter(t, err)

			// Expect that the backend lock was NOT updated
			updated, err := srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
			require.Equal(t, time.Time{}, updated.Expiry())
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Run("okta service deleting okta-origin lock is allowed", func(t *testing.T) {
			// Given an existing okta-origin lock in the cluster...
			targetUser := newUserInCluster(t, types.OriginOkta)
			lock := newLockInCluster(t, targetUser, types.OriginOkta)

			// When I attempt to delete that lock...
			err = authWithOktaRole.DeleteLock(ctx, lock.GetName())

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect no such lock to exist
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			requireNotFound(t, err)
		})

		t.Run("okta service deleting non-okta-origin lock is an error", func(t *testing.T) {
			// Given an existing non-okta-origin lock in the cluster...
			targetUser := newUserInCluster(t, "")
			lock := newLockInCluster(t, targetUser, "")

			// When the okta server attempts to delete that lock...
			err = authWithOktaRole.DeleteLock(ctx, lock.GetName())

			// Expect the operation to fail with AccessDenied
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err), "Expected AccessDenied, got %T: %s", err, err.Error())

			// Expect that the lock still exists
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			require.NoError(t, err)
		})

		t.Run("non-okta service deleting non-okta-origin lock is allowed", func(t *testing.T) {
			// Given an existing okta-origin lock in the cluster...
			targetUser := newUserInCluster(t, types.OriginOkta)
			lock := newLockInCluster(t, targetUser, types.OriginOkta)

			// When a non-okta service attempts to delete that lock...
			err = authWithAdminRole.DeleteLock(ctx, lock.GetName())

			// Expect the operation to succeed
			require.NoError(t, err)

			// Expect no such lock to exist
			_, err = srv.AuthServer.GetLock(ctx, lock.GetName())
			requireNotFound(t, err)
		})
	})
}
