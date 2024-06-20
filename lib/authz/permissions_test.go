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

package authz

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const clusterName = "test-cluster"

func TestGetDisconnectExpiredCertFromIdentity(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now()
	inAnHour := clock.Now().Add(time.Hour)

	for _, test := range []struct {
		name                    string
		expires                 time.Time
		previousIdentityExpires time.Time
		checker                 services.AccessChecker
		mfaVerified             bool
		disconnectExpiredCert   bool
		expected                time.Time
	}{
		{
			name:                    "mfa overrides expires when set",
			checker:                 &fakeCtxChecker{},
			expires:                 now,
			previousIdentityExpires: inAnHour,
			mfaVerified:             true,
			disconnectExpiredCert:   true,
			expected:                inAnHour,
		},
		{
			name:                  "expires returned when mfa unset",
			checker:               &fakeCtxChecker{},
			expires:               now,
			mfaVerified:           false,
			disconnectExpiredCert: true,
			expected:              now,
		},
		{
			name:                    "unset when disconnectExpiredCert is false",
			checker:                 &fakeCtxChecker{},
			expires:                 now,
			previousIdentityExpires: inAnHour,
			mfaVerified:             true,
			disconnectExpiredCert:   false,
		},
		{
			name:                  "no expiry returned when checker nil and disconnectExpiredCert false",
			checker:               nil,
			expires:               now,
			mfaVerified:           false,
			disconnectExpiredCert: false,
			expected:              time.Time{},
		},
		{
			name:                  "expiry returned when checker nil and disconnectExpiredCert true",
			checker:               nil,
			expires:               now,
			mfaVerified:           false,
			disconnectExpiredCert: true,
			expected:              now,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var mfaVerified string
			if test.mfaVerified {
				mfaVerified = "1234"
			}
			identity := tlsca.Identity{
				Expires:                 test.expires,
				PreviousIdentityExpires: test.previousIdentityExpires,
				MFAVerified:             mfaVerified,
			}

			authPref := types.DefaultAuthPreference()
			authPref.SetDisconnectExpiredCert(test.disconnectExpiredCert)

			ctx := Context{Checker: test.checker, Identity: WrapIdentity(identity)}

			got := ctx.GetDisconnectCertExpiry(authPref)
			require.Equal(t, test.expected, got)
		})
	}
}

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
				{Device: "device1"},
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
				{Device: "device1"},
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
				{Device: "device1"},
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
				{Device: "device1"},
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
				{Device: "device1"},
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
						DeviceExtensions: tlsca.DeviceExtensions{
							DeviceID: "device1",
						},
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

	_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Remove the MFA record from the user value being authorized.
	localUser.Identity.MFAVerified = ""
	_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, localUser))
	require.NoError(t, err)

	// Add an access request lock.
	requestLock, err := types.NewLock("request-lock", types.LockSpecV2{
		Target: types.LockTarget{AccessRequest: localUser.Identity.ActiveRequests[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, requestLock)

	// localUser's identity with a locked access request is locked out.
	_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Not locked out without the request.
	localUser.Identity.ActiveRequests = nil
	_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, localUser))
	require.NoError(t, err)

	// Create a lock targeting the role written in the user's identity.
	roleLock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: localUser.Identity.Groups[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, roleLock)

	_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, localUser))
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

			_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, builtinRole))
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))

			builtinRole.Identity.Username = ""
			_, err = authorizer.Authorize(context.WithValue(ctx, contextUser, builtinRole))
			require.NoError(t, err)
		})
	}
}

func upsertLockWithPutEvent(ctx context.Context, t *testing.T, client *testClient, watcher *services.LockWatcher, lock types.Lock) {
	lockWatch, err := watcher.Subscribe(ctx)
	require.NoError(t, err)
	defer lockWatch.Close()

	require.NoError(t, client.UpsertLock(ctx, lock))

	// Retry a few times to wait for the resource event we expect as the
	// resource watcher can potentially return events for previously
	// created resources as well.
	require.Eventually(t, func() bool {
		select {
		case event := <-lockWatch.Events():
			return types.OpPut == event.Type && resourceDiff(lock, event.Resource) == ""
		case <-lockWatch.Done():
			return false
		}
	}, 2*time.Second, 100*time.Millisecond)
}

func TestGetClientUserIsSSO(t *testing.T) {
	ctx := context.Background()

	u := LocalUser{
		Username: "someuser",
		Identity: tlsca.Identity{
			Username: "someuser",
			Groups:   []string{"somerole"},
		},
	}

	// Non SSO user must return false
	nonSSOUserCtx := context.WithValue(ctx, contextUser, u)

	isSSO, err := GetClientUserIsSSO(nonSSOUserCtx)
	require.NoError(t, err)
	require.False(t, isSSO, "expected a non-SSO user")

	// An SSO user must return true
	u.Identity.UserType = types.UserTypeSSO
	ssoUserCtx := context.WithValue(ctx, contextUser, u)
	localUserIsSSO, err := GetClientUserIsSSO(ssoUserCtx)
	require.NoError(t, err)
	require.True(t, localUserIsSSO, "expected an SSO user")
}

func TestAuthorizer_Authorize_deviceTrust(t *testing.T) {
	client, watcher, _ := newTestResources(t)

	ctx := context.Background()

	user, role, err := createUserAndRole(client, "llama", []string{"llama"}, nil)
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
			wantErr:    "access denied",
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
				ClusterName: clusterName,
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
				ClusterName: clusterName,
				Identity:    userWithoutExtensions.Identity,
			},
		},
		{
			name:               "RemoteBuiltinRole: device authorization disabled",
			disableDeviceAuthz: true,
			user: RemoteBuiltinRole{
				Role:        types.RoleProxy,
				Username:    user.GetName(),
				ClusterName: clusterName,
				Identity:    userWithoutExtensions.Identity,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Update device trust mode.
			authPref, err := client.GetAuthPreference(ctx)
			require.NoError(t, err, "GetAuthPreference failed")
			apV2 := authPref.(*types.AuthPreferenceV2)
			apV2.Spec.DeviceTrust = &types.DeviceTrust{
				Mode: test.deviceMode,
			}
			require.NoError(t,
				client.SetAuthPreference(ctx, apV2),
				"SetAuthPreference failed")

			// Create a new authorizer.
			authorizer, err := NewAuthorizer(AuthorizerOpts{
				ClusterName:                clusterName,
				AccessPoint:                client,
				LockWatcher:                watcher,
				DisableDeviceAuthorization: test.disableDeviceAuthz,
			})
			require.NoError(t, err, "NewAuthorizer failed")

			// Test!
			userCtx := context.WithValue(ctx, contextUser, test.user)
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

func TestCheckIPPinning(t *testing.T) {
	testCases := []struct {
		desc       string
		clientAddr string
		pinnedIP   string
		pinIP      bool
		wantErr    string
	}{
		{
			desc:       "no IP pinning",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "",
			pinIP:      false,
		},
		{
			desc:       "IP pinning, no pinned IP",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "",
			pinIP:      true,
			wantErr:    ErrIPPinningMissing.Error(),
		},
		{
			desc:       "Pinned IP doesn't match",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.2",
			pinIP:      true,
			wantErr:    ErrIPPinningMismatch.Error(),
		},
		{
			desc:       "Role doesn't require IP pinning now, but old certificate still pinned",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.2",
			pinIP:      false,
			wantErr:    ErrIPPinningMismatch.Error(),
		},
		{
			desc:     "IP pinning enabled, missing client IP",
			pinnedIP: "127.0.0.1",
			pinIP:    true,
			wantErr:  "client source address was not found in the context",
		},
		{
			desc:       "IP pinning enabled, port=0 (marked by proxyProtocolMode unspecified)",
			clientAddr: "127.0.0.1:0",
			pinnedIP:   "127.0.0.1",
			pinIP:      true,
			wantErr:    ErrIPPinningMismatch.Error(),
		},
		{
			desc:       "correct IP pinning",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.1",
			pinIP:      true,
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		if tt.clientAddr != "" {
			ctx = ContextWithClientSrcAddr(ctx, utils.MustParseAddr(tt.clientAddr))
		}
		identity := tlsca.Identity{PinnedIP: tt.pinnedIP}

		err := CheckIPPinning(ctx, identity, tt.pinIP, nil)

		if tt.wantErr != "" {
			require.ErrorContains(t, err, tt.wantErr)
		} else {
			require.NoError(t, err)
		}

	}
}

func TestRoleSetForBuiltinRoles(t *testing.T) {
	tests := []struct {
		name          string
		clusterName   string
		recConfig     types.SessionRecordingConfig
		roles         []types.SystemRole
		assertRoleSet func(t *testing.T, rs services.RoleSet)
	}{
		{
			name:        "RoleMDM is mapped",
			clusterName: clusterName,
			roles:       []types.SystemRole{types.RoleMDM},
			assertRoleSet: func(t *testing.T, rs services.RoleSet) {
				for i, r := range rs {
					assert.NotEmpty(t, r.GetNamespaces(types.Allow), "RoleSetForBuiltinRoles: rs[%v]: role has no namespaces", i)
					assert.NotEmpty(t, r.GetRules(types.Allow), "RoleSetForBuiltinRoles: rs[%v]: role has no rules", i)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rs, err := RoleSetForBuiltinRoles(test.clusterName, test.recConfig, test.roles...)
			require.NoError(t, err, "RoleSetForBuiltinRoles failed")
			assert.NotEmpty(t, rs, "RoleSetForBuiltinRoles returned a nil RoleSet")
			test.assertRoleSet(t, rs)
		})
	}
}

func TestConnectionMetadata(t *testing.T) {
	for name, test := range map[string]struct {
		ctx                        context.Context
		expectedConnectionMetadata apievents.ConnectionMetadata
	}{
		"with client address": {
			ctx:                        ContextWithClientSrcAddr(context.Background(), &net.TCPAddr{IP: net.IPv4(10, 255, 0, 0), Port: 1234}),
			expectedConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: "10.255.0.0:1234"},
		},
		"empty client address": {
			ctx:                        context.Background(),
			expectedConnectionMetadata: apievents.ConnectionMetadata{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, cmp.Diff(test.expectedConnectionMetadata, ConnectionMetadata(test.ctx)))
		})
	}
}

func TestIsUserFunctions(t *testing.T) {
	localIdentity := Context{Identity: LocalUser{}}
	remoteIdentity := Context{Identity: RemoteUser{}}
	systemIdentity := Context{
		Identity: BuiltinRole{
			Role: types.RoleProxy,
		},
	}

	tests := []struct {
		funcName, scenario string
		isUserFunc         func(Context) bool
		authCtx            Context
		want               bool
	}{
		{
			funcName:   "IsLocalUser",
			scenario:   "local user",
			isUserFunc: IsLocalUser,
			authCtx:    localIdentity,
			want:       true,
		},
		{
			funcName:   "IsLocalUser",
			scenario:   "remote user",
			isUserFunc: IsLocalUser,
			authCtx:    remoteIdentity,
		},
		{
			funcName:   "IsLocalUser",
			scenario:   "system user",
			isUserFunc: IsLocalUser,
			authCtx:    systemIdentity,
		},
		{
			funcName:   "IsRemoteUser",
			scenario:   "local user",
			isUserFunc: IsRemoteUser,
			authCtx:    localIdentity,
		},
		{
			funcName:   "IsRemoteUser",
			scenario:   "remote user",
			isUserFunc: IsRemoteUser,
			authCtx:    remoteIdentity,
			want:       true,
		},
		{
			funcName:   "IsRemoteUser",
			scenario:   "system user",
			isUserFunc: IsRemoteUser,
			authCtx:    systemIdentity,
		},

		{
			funcName:   "IsLocalOrRemoteUser",
			scenario:   "local user",
			isUserFunc: IsLocalOrRemoteUser,
			authCtx:    localIdentity,
			want:       true,
		},
		{
			funcName:   "IsLocalOrRemoteUser",
			scenario:   "remote user",
			isUserFunc: IsLocalOrRemoteUser,
			authCtx:    remoteIdentity,
			want:       true,
		},
		{
			funcName:   "IsLocalOrRemoteUser",
			scenario:   "system user",
			isUserFunc: IsLocalOrRemoteUser,
			authCtx:    systemIdentity,
		},
	}
	for _, test := range tests {
		t.Run(test.funcName+"/"+test.scenario, func(t *testing.T) {
			got := test.isUserFunc(test.authCtx)
			assert.Equal(t, test.want, got, "%s mismatch", test.funcName)
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

func (c *fakeCtxChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	return disconnect
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
	identitySvc := local.NewTestIdentityService(backend)
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

	authorizer, err := NewAuthorizer(AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: client,
		LockWatcher: lockWatcher,
	})
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

func resourceDiff(res1, res2 types.Resource) string {
	return cmp.Diff(res1, res2,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
		cmpopts.EquateEmpty())
}
