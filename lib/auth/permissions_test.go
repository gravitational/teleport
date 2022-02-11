/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestContextLockTargets(t *testing.T) {
	t.Parallel()
	authContext := &Context{
		Identity: BuiltinRole{
			Role:        types.RoleNode,
			ClusterName: "cluster",
			Identity: tlsca.Identity{
				Username: "node.cluster",
				Groups:   []string{"role1", "role2"},
			},
		},
		UnmappedIdentity: WrapIdentity(tlsca.Identity{
			Username: "node.cluster",
			Groups:   []string{"mapped-role"},
		}),
	}
	expected := []types.LockTarget{
		{Node: "node"},
		{Node: "node.cluster"},
		{User: "node.cluster"},
		{Role: "role1"},
		{Role: "role2"},
		{Role: "mapped-role"},
	}
	require.ElementsMatch(t, authContext.LockTargets(), expected)
}

func TestAuthorizeWithLocksForLocalUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(srv.AuthServer, "test-user", []string{})
	require.NoError(t, err)
	localUser := LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username:       user.GetName(),
			Groups:         []string{"test-role-1"},
			MFAVerified:    "mfa-device-id",
			ActiveRequests: []string{"test-request"},
		},
	}

	// Apply an MFA lock.
	mfaLock, err := types.NewLock("mfa-lock", types.LockSpecV2{
		Target: types.LockTarget{MFADevice: localUser.Identity.MFAVerified},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, srv, mfaLock)

	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Remove the MFA record from the user value being authorized.
	localUser.Identity.MFAVerified = ""
	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.NoError(t, err)

	// Add an access request lock.
	requestLock, err := types.NewLock("request-lock", types.LockSpecV2{
		Target: types.LockTarget{AccessRequest: localUser.Identity.ActiveRequests[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, srv, requestLock)

	// localUser's identity with a locked access request is locked out.
	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Not locked out without the request.
	localUser.Identity.ActiveRequests = nil
	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.NoError(t, err)

	// Create a lock targeting the role written in the user's identity.
	roleLock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: localUser.Identity.Groups[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, srv, roleLock)

	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestAuthorizeWithLocksForBuiltinRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	builtinRole := BuiltinRole{
		Username: "node",
		Role:     types.RoleNode,
		Identity: tlsca.Identity{
			Username: "node",
		},
	}

	// Apply a node lock.
	nodeLock, err := types.NewLock("node-lock", types.LockSpecV2{
		Target: types.LockTarget{Node: builtinRole.Identity.Username},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, srv, nodeLock)

	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, builtinRole))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	builtinRole.Identity.Username = ""
	_, err = srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, builtinRole))
	require.NoError(t, err)
}

func upsertLockWithPutEvent(ctx context.Context, t *testing.T, srv *TestAuthServer, lock types.Lock) {
	lockWatch, err := srv.LockWatcher.Subscribe(ctx)
	require.NoError(t, err)
	defer lockWatch.Close()

	require.NoError(t, srv.AuthServer.UpsertLock(ctx, lock))
	select {
	case event := <-lockWatch.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Empty(t, resourceDiff(lock, event.Resource))
	case <-lockWatch.Done():
		t.Fatalf("Watcher exited with error: %v.", lockWatch.Error())
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for lock put.")
	}
}
