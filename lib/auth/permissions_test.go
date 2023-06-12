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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const clusterName = "test-cluster"

func TestContextLockTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role types.SystemRole
		want []types.LockTarget
	}{
		{
			role: types.RoleNode,
			want: []types.LockTarget{
				{Node: "node", ServerID: "node"},
				{Node: "node.cluster", ServerID: "node.cluster"},
				{User: "node.cluster"},
				{Role: "role1"},
				{Role: "role2"},
				{Role: "mapped-role"},
			},
		},
		{
			role: types.RoleAuth,
			want: []types.LockTarget{
				{ServerID: "node"},
				{ServerID: "node.cluster"},
				{User: "node.cluster"},
				{Role: "role1"},
				{Role: "role2"},
				{Role: "mapped-role"},
			},
		},
		{
			role: types.RoleProxy,
			want: []types.LockTarget{
				{ServerID: "node"},
				{ServerID: "node.cluster"},
				{User: "node.cluster"},
				{Role: "role1"},
				{Role: "role2"},
				{Role: "mapped-role"},
			},
		},
		{
			role: types.RoleKube,
			want: []types.LockTarget{
				{ServerID: "node"},
				{ServerID: "node.cluster"},
				{User: "node.cluster"},
				{Role: "role1"},
				{Role: "role2"},
				{Role: "mapped-role"},
			},
		},
		{
			role: types.RoleDatabase,
			want: []types.LockTarget{
				{ServerID: "node"},
				{ServerID: "node.cluster"},
				{User: "node.cluster"},
				{Role: "role1"},
				{Role: "role2"},
				{Role: "mapped-role"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.role.String(), func(t *testing.T) {
			authContext := &Context{
				Identity: BuiltinRole{
					Role:        tt.role,
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
			require.ElementsMatch(t, authContext.LockTargets(), tt.want)
		})
	}
}

func TestAuthorizeWithLocksForLocalUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client, watcher, authorizer := newTestResources(t)

	user, role, err := createUserAndRole(client, "test-user", []string{}, nil)
	require.NoError(t, err)
	localUser := LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username:       user.GetName(),
			Groups:         []string{role.GetName()},
			MFAVerified:    "mfa-device-id",
			ActiveRequests: []string{"test-request"},
		},
	}

	// Apply an MFA lock.
	mfaLock, err := types.NewLock("mfa-lock", types.LockSpecV2{
		Target: types.LockTarget{MFADevice: localUser.Identity.MFAVerified},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, mfaLock)

	_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Remove the MFA record from the user value being authorized.
	localUser.Identity.MFAVerified = ""
	_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.NoError(t, err)

	// Add an access request lock.
	requestLock, err := types.NewLock("request-lock", types.LockSpecV2{
		Target: types.LockTarget{AccessRequest: localUser.Identity.ActiveRequests[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, requestLock)

	// localUser's identity with a locked access request is locked out.
	_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Not locked out without the request.
	localUser.Identity.ActiveRequests = nil
	_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.NoError(t, err)

	// Create a lock targeting the role written in the user's identity.
	roleLock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: localUser.Identity.Groups[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, roleLock)

	_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestAuthorizeWithLocksForBuiltinRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client, watcher, authorizer := newTestResources(t)
	for _, role := range types.LocalServiceMappings() {
		t.Run(role.String(), func(t *testing.T) {
			builtinRole := BuiltinRole{
				Username: "node",
				Role:     role,
				Identity: tlsca.Identity{
					Username: "node",
				},
			}

			// Apply a node lock.
			nodeLock, err := types.NewLock("node-lock", types.LockSpecV2{
				Target: types.LockTarget{ServerID: builtinRole.Identity.Username},
			})
			require.NoError(t, err)
			upsertLockWithPutEvent(ctx, t, client, watcher, nodeLock)

			_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, builtinRole))
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))

			builtinRole.Identity.Username = ""
			_, err = authorizer.Authorize(context.WithValue(ctx, ContextUser, builtinRole))
			require.NoError(t, err)
		})
	}
}

func upsertLockWithPutEvent(ctx context.Context, t *testing.T, client *testClient, watcher *services.LockWatcher, lock types.Lock) {
	lockWatch, err := watcher.Subscribe(ctx)
	require.NoError(t, err)
	defer lockWatch.Close()

	require.NoError(t, client.UpsertLock(ctx, lock))
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

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.Access
	services.Identity
	types.Events
}

func newTestResources(t *testing.T) (*testClient, *services.LockWatcher, Authorizer) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, backend.Close())
	})

	clusterConfig, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	caSvc := local.NewCAService(backend)
	accessSvc := local.NewAccessService(backend)
	identitySvc := local.NewIdentityService(backend)
	eventsSvc := local.NewEventsService(backend)

	client := &testClient{
		ClusterConfiguration: clusterConfig,
		Trust:                caSvc,
		Access:               accessSvc,
		Identity:             identitySvc,
		Events:               eventsSvc,
	}

	// Set default singletons
	ctx := context.Background()
	client.SetAuthPreference(ctx, types.DefaultAuthPreference())
	client.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig())
	client.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	client.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())

	lockSvc := local.NewAccessService(backend)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
		LockGetter: lockSvc,
	})
	require.NoError(t, err)

	authorizer, err := NewAuthorizer(clusterName, client, lockWatcher)
	require.NoError(t, err)

	return client, lockWatcher, authorizer
}

func createUserAndRole(client *testClient, username string, allowedLogins []string, allowRules []types.Rule) (types.User, types.Role, error) {
	ctx := context.Background()
	user, err := types.NewUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	role := services.RoleForUser(user)
	role.SetLogins(types.Allow, allowedLogins)
	if allowRules != nil {
		role.SetRules(types.Allow, allowRules)
	}

	err = client.UpsertRole(ctx, role)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	user.AddRole(role.GetName())
	err = client.UpsertUser(user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return user, role, nil
}
