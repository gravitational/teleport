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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
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

	user, role, err := CreateUserAndRole(srv.AuthServer, "test-user", []string{})
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

func TestAuthorizer_Authorize_deviceTrust(t *testing.T) {
	t.Parallel()

	testServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err, "NewTestAuthServer failed")

	authServer := testServer.AuthServer
	ctx := context.Background()

	user, role, err := CreateUserAndRole(authServer, "llama", []string{"llama"})
	require.NoError(t, err, "CreateUserAndRole")

	userWithoutExtensions := LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username:   user.GetName(),
			Groups:     []string{role.GetName()},
			Principals: user.GetLogins(),
		},
	}
	userWithExtensions := userWithoutExtensions
	userWithExtensions.Identity.DeviceExtensions = tlsca.DeviceExtensions{
		DeviceID:     "deviceid1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	// Enterprise is necessary for mode=optional and mode=required to work.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	tests := []struct {
		name                 string
		deviceMode           string
		disableDeviceAuthz   bool
		user                 IdentityGetter
		wantErr              string
		wantCtxAuthnDisabled bool // defaults to disableDeviceAuthz
	}{
		{
			name:       "user without extensions and mode=off",
			deviceMode: constants.DeviceTrustModeOff,
			user:       userWithoutExtensions,
		},
		{
			name:       "nok: user without extensions and mode=required",
			deviceMode: constants.DeviceTrustModeRequired,
			user:       userWithoutExtensions,
			wantErr:    "unauthorized device",
		},
		{
			name:               "device authorization disabled",
			deviceMode:         constants.DeviceTrustModeRequired,
			disableDeviceAuthz: true,
			user:               userWithoutExtensions,
		},
		{
			name:       "user with extensions and mode=required",
			deviceMode: constants.DeviceTrustModeRequired,
			user:       userWithExtensions,
		},
		{
			name: "BuiltinRole: context always disabled",
			user: BuiltinRole{
				Role:        types.RoleProxy,
				Username:    user.GetName(),
				ClusterName: testServer.ClusterName,
				Identity:    userWithoutExtensions.Identity,
			},
			wantCtxAuthnDisabled: true, // BuiltinRole ctx validation disabled by default
		},
		{
			name:               "BuiltinRole: device authorization disabled",
			disableDeviceAuthz: true,
			user: BuiltinRole{
				Role:        types.RoleProxy,
				Username:    user.GetName(),
				ClusterName: testServer.ClusterName,
				Identity:    userWithoutExtensions.Identity,
			},
		},
		{
			name:               "RemoteBuiltinRole: device authorization disabled",
			disableDeviceAuthz: true,
			user: RemoteBuiltinRole{
				Role:        types.RoleProxy,
				Username:    user.GetName(),
				ClusterName: testServer.ClusterName,
				Identity:    userWithoutExtensions.Identity,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Update device trust mode.
			authPref, err := authServer.GetAuthPreference(ctx)
			require.NoError(t, err, "GetAuthPreference failed")
			apV2 := authPref.(*types.AuthPreferenceV2)
			apV2.Spec.DeviceTrust = &types.DeviceTrust{
				Mode: test.deviceMode,
			}
			require.NoError(t,
				authServer.SetAuthPreference(ctx, apV2),
				"SetAuthPreference failed")

			// Create a new authorizer.
			authorizer, err := NewAuthorizer(AuthorizerOpts{
				ClusterName:                testServer.ClusterName,
				AccessPoint:                authServer,
				LockWatcher:                testServer.LockWatcher,
				DisableDeviceAuthorization: test.disableDeviceAuthz,
			})
			require.NoError(t, err, "NewAuthorizer failed")

			// Test!
			userCtx := context.WithValue(ctx, ContextUser, test.user)
			authCtx, gotErr := authorizer.Authorize(userCtx)
			if test.wantErr == "" {
				assert.NoError(t, gotErr, "Authorize returned unexpected error")
			} else {
				assert.ErrorContains(t, gotErr, test.wantErr, "Authorize mismatch")
				assert.True(t, trace.IsAccessDenied(gotErr), "Authorize returned err=%T, want trace.AccessDeniedError", gotErr)
			}
			if gotErr != nil {
				return
			}

			// Verify that the auth.Context has the correct disableDeviceAuthorization
			// value.
			wantDisabled := test.disableDeviceAuthz || test.wantCtxAuthnDisabled
			assert.Equal(
				t, wantDisabled, authCtx.disableDeviceAuthorization,
				"auth.Context.disableDeviceAuthorization not inherited from Authorizer")
		})
	}
}

func TestContext_GetAccessState(t *testing.T) {
	localCtx := Context{
		User:    &fakeCtxUser{}, // makes no difference in the outcomes.
		Checker: &fakeCtxChecker{},
		Identity: LocalUser{Identity: tlsca.Identity{
			Username:   "llama",
			Groups:     []string{"access", "editor", "llamas"},
			Principals: []string{"llamas"},
		}},
	}

	deviceExt := tlsca.DeviceExtensions{
		DeviceID:     "deviceid1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	defaultSpec := &types.AuthPreferenceSpecV2{}

	tests := []struct {
		name          string
		createAuthCtx func() *Context
		authSpec      *types.AuthPreferenceSpecV2 // defaults to defaultSpec
		want          services.AccessState
	}{
		{
			name:          "local user",
			createAuthCtx: func() *Context { return &localCtx },
			want: services.AccessState{
				EnableDeviceVerification: true, // default when acquired from auth.Context
			},
		},
		{
			name: "builtin role",
			createAuthCtx: func() *Context {
				ctx := localCtx
				ctx.Identity = BuiltinRole{}
				return &ctx
			},
			want: services.AccessState{
				MFAVerified:              true, // builtin roles are always verified
				EnableDeviceVerification: true, // default
				DeviceVerified:           true, // builtin roles are always verified
			},
		},
		{
			name: "mfa: local user",
			createAuthCtx: func() *Context {
				ctx := localCtx
				ctx.Checker = &fakeCtxChecker{state: services.AccessState{
					MFARequired: services.MFARequiredAlways,
				}}
				localUser := ctx.Identity.(LocalUser)
				localUser.Identity.MFAVerified = "my-device-UUID"
				ctx.Identity = localUser
				return &ctx
			},
			want: services.AccessState{
				MFARequired:              services.MFARequiredAlways, // copied from AccessChecker
				MFAVerified:              true,                       // copied from Identity
				EnableDeviceVerification: true,
			},
		},
		{
			name: "device trust: local user",
			createAuthCtx: func() *Context {
				ctx := localCtx
				localUser := ctx.Identity.(LocalUser)
				localUser.Identity.DeviceExtensions = deviceExt
				ctx.Identity = localUser
				return &ctx
			},
			want: services.AccessState{
				EnableDeviceVerification: true,
				DeviceVerified:           true, // Identity extensions
			},
		},
		{
			name: "device authorization disabled",
			createAuthCtx: func() *Context {
				ctx := localCtx
				localUser := ctx.Identity.(LocalUser)
				localUser.Identity.DeviceExtensions = deviceExt
				ctx.Identity = localUser
				ctx.disableDeviceAuthorization = true
				return &ctx
			},
			want: services.AccessState{
				EnableDeviceVerification: false, // copied from Context
				DeviceVerified:           true,  // Identity extensions
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// Prepare AuthPreference.
			spec := test.authSpec
			if spec == nil {
				spec = defaultSpec
			}
			authPref, err := types.NewAuthPreference(*spec)
			require.NoError(t, err, "NewAuthPreference failed")

			// Test!
			got := test.createAuthCtx().GetAccessState(authPref)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("GetAccessState mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

// fakeCtxUser is used for auth.Context tests.
type fakeCtxUser struct {
	types.User
}

// fakeCtxChecker is used for auth.Context tests.
type fakeCtxChecker struct {
	services.AccessChecker
	state services.AccessState
}

func (c *fakeCtxChecker) GetAccessState(_ types.AuthPreference) services.AccessState {
	return c.state
}
