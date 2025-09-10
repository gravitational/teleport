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

package authz_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	clusterName = "test-cluster"
)

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

			ctx := authz.Context{Checker: test.checker, Identity: authz.WrapIdentity(identity)}

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
			authContext := &authz.Context{
				Identity: authz.BuiltinRole{
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
				UnmappedIdentity: authz.WrapIdentity(tlsca.Identity{
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

	user, role, err := authtest.CreateUserAndRole(client, "test-user", []string{}, nil)
	require.NoError(t, err)
	localUser := authz.LocalUser{
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

	_, err = authorizer.Authorize(authz.ContextWithUser(ctx, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Remove the MFA record from the user value being authorized.
	localUser.Identity.MFAVerified = ""
	_, err = authorizer.Authorize(authz.ContextWithUser(ctx, localUser))
	require.NoError(t, err)

	// Add an access request lock.
	requestLock, err := types.NewLock("request-lock", types.LockSpecV2{
		Target: types.LockTarget{AccessRequest: localUser.Identity.ActiveRequests[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, requestLock)

	// localUser's identity with a locked access request is locked out.
	_, err = authorizer.Authorize(authz.ContextWithUser(ctx, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Not locked out without the request.
	localUser.Identity.ActiveRequests = nil
	_, err = authorizer.Authorize(authz.ContextWithUser(ctx, localUser))
	require.NoError(t, err)

	// Create a lock targeting the role written in the user's identity.
	roleLock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: localUser.Identity.Groups[0]},
	})
	require.NoError(t, err)
	upsertLockWithPutEvent(ctx, t, client, watcher, roleLock)

	_, err = authorizer.Authorize(authz.ContextWithUser(ctx, localUser))
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestAuthorizeWithLocksForBuiltinRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client, watcher, authorizer := newTestResources(t)
	for _, role := range types.LocalServiceMappings() {
		t.Run(role.String(), func(t *testing.T) {
			builtinRole := authz.BuiltinRole{
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

			_, err = authorizer.Authorize(authz.ContextWithUser(ctx, builtinRole))
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))

			builtinRole.Identity.Username = ""
			_, err = authorizer.Authorize(authz.ContextWithUser(ctx, builtinRole))
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

	u := authz.LocalUser{
		Username: "someuser",
		Identity: tlsca.Identity{
			Username: "someuser",
			Groups:   []string{"somerole"},
		},
	}

	// Non SSO user must return false
	nonSSOUserCtx := authz.ContextWithUser(ctx, u)

	isSSO, err := authz.GetClientUserIsSSO(nonSSOUserCtx)
	require.NoError(t, err)
	require.False(t, isSSO, "expected a non-SSO user")

	// An SSO user must return true
	u.Identity.UserType = types.UserTypeSSO
	ssoUserCtx := authz.ContextWithUser(ctx, u)
	localUserIsSSO, err := authz.GetClientUserIsSSO(ssoUserCtx)
	require.NoError(t, err)
	require.True(t, localUserIsSSO, "expected an SSO user")
}

func TestAuthorizer_Authorize_deviceTrust(t *testing.T) {
	client, watcher, _ := newTestResources(t)

	ctx := context.Background()

	user, role, err := authtest.CreateUserAndRole(client, "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole")

	userWithoutExtensions := authz.LocalUser{
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
	botUser := userWithoutExtensions
	botUser.Identity.BotName = "wall-e"
	botUser.Identity.BotInstanceID = uuid.NewString()

	// Enterprise is necessary for mode=optional and mode=required to work.
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
	})

	tests := []struct {
		name                 string
		deviceMode           string
		deviceAuthz          authz.DeviceAuthorizationOpts // aka AuthorizerOpts.DeviceAuthorization
		user                 authz.IdentityGetter
		wantErr              string
		wantCtxAuthnDisabled bool // defaults to deviceAuthz.disableDeviceRoleMode
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
			name:       "nok: bot user and mode=required",
			deviceMode: constants.DeviceTrustModeRequired,
			user:       botUser,
			wantErr:    "access denied",
		},
		{
			name:       "ok: bot user and mode=required-for-humans",
			deviceMode: constants.DeviceTrustModeRequiredForHumans,
			user:       botUser,
		},
		{
			name:       "global mode disabled only",
			deviceMode: constants.DeviceTrustModeRequired,
			deviceAuthz: authz.DeviceAuthorizationOpts{
				DisableGlobalMode: true,
			},
			user: userWithoutExtensions,
		},
		{
			name:       "global and role modes disabled",
			deviceMode: constants.DeviceTrustModeRequired,
			deviceAuthz: authz.DeviceAuthorizationOpts{
				DisableGlobalMode: true,
				DisableRoleMode:   true,
			},
			user: userWithoutExtensions,
		},
		{
			name:       "user with extensions and mode=required",
			deviceMode: constants.DeviceTrustModeRequired,
			user:       userWithExtensions,
		},
		{
			name: "BuiltinRole: context always disabled",
			user: authz.BuiltinRole{
				Role:        types.RoleProxy,
				Username:    user.GetName(),
				ClusterName: clusterName,
				Identity:    userWithoutExtensions.Identity,
			},
			wantCtxAuthnDisabled: true, // BuiltinRole ctx validation disabled by default
		},
		{
			name: "BuiltinRole: device authorization disabled",
			deviceAuthz: authz.DeviceAuthorizationOpts{
				DisableGlobalMode: true,
				DisableRoleMode:   true,
			},
			user: authz.BuiltinRole{
				Role:        types.RoleProxy,
				Username:    user.GetName(),
				ClusterName: clusterName,
				Identity:    userWithoutExtensions.Identity,
			},
		},
		{
			name: "RemoteBuiltinRole: device authorization disabled",
			deviceAuthz: authz.DeviceAuthorizationOpts{
				DisableGlobalMode: true,
				DisableRoleMode:   true,
			},
			user: authz.RemoteBuiltinRole{
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
			_, err = client.UpsertAuthPreference(ctx, apV2)
			require.NoError(t, err, "UpsertAuthPreference failed")

			// Create a new authorizer.
			authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
				ClusterName:         clusterName,
				AccessPoint:         client,
				LockWatcher:         watcher,
				DeviceAuthorization: test.deviceAuthz,
			})
			require.NoError(t, err, "NewAuthorizer failed")

			// Test!
			userCtx := authz.ContextWithUser(ctx, test.user)
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
			// value, based on either the global toggle or role mode.
			wantDisabled := test.deviceAuthz.DisableRoleMode || test.wantCtxAuthnDisabled
			assert.Equal(
				t, wantDisabled, authz.ContextDeviceRoleMode(authCtx),
				"auth.Context.disableDeviceAuthorization not inherited from Authorizer")
		})
	}
}

// hostFQDN consists of host UUID and cluster name joined via .
func hostFQDN(hostUUID, clusterName string) string {
	return fmt.Sprintf("%v.%v", hostUUID, clusterName)
}

type fakeMFAAuthenticator struct {
	mfaData map[string]*authz.MFAAuthData // keyed by totp token
}

func (a *fakeMFAAuthenticator) ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
	mfaData, ok := a.mfaData[resp.GetTOTP().GetCode()]
	if !ok {
		return nil, trace.AccessDenied("invalid MFA")
	}
	return mfaData, nil
}

func TestAuthorizer_AuthorizeAdminAction(t *testing.T) {
	ctx := context.Background()
	client, watcher, _ := newTestResources(t)

	// Enable Webauthn.
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = client.UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	// Create a new local user.
	localUser, _, err := authtest.CreateUserAndRole(client, "localuser", []string{"local"}, nil)
	require.NoError(t, err)

	// Create new local user with a host-like username.
	userWithHostName, _, err := authtest.CreateUserAndRole(client, hostFQDN(uuid.NewString(), clusterName), []string{"local"}, nil)
	require.NoError(t, err)

	// Create a new bot user.
	bot, _, err := authtest.CreateUserAndRole(client, "robot", []string{"robot-role"}, nil)
	require.NoError(t, err)
	botMetadata := bot.GetMetadata()
	botMetadata.Labels = map[string]string{
		types.BotLabel:           bot.GetName(),
		types.BotGenerationLabel: "0",
	}
	bot.SetMetadata(botMetadata)
	_, err = client.UpsertUser(ctx, bot)
	require.NoError(t, err)

	validTOTPCode := "valid"
	validReusableTOTPCode := "valid-reusable"
	fakeMFAAuthentictor := &fakeMFAAuthenticator{
		mfaData: map[string]*authz.MFAAuthData{
			validTOTPCode: {},
			validReusableTOTPCode: {
				AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
			},
		},
	}

	// Create a new authorizer.
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName:      clusterName,
		AccessPoint:      client,
		LockWatcher:      watcher,
		MFAAuthenticator: fakeMFAAuthentictor,
	})
	require.NoError(t, err, "NewAuthorizer failed")

	validMFA := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: validTOTPCode,
			},
		},
	}

	validMFAWithReuse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: validReusableTOTPCode,
			},
		},
	}

	invalidMFA := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: "invalid",
			},
		},
	}

	for _, tt := range []struct {
		name                      string
		user                      authz.IdentityGetter
		withMFA                   *proto.MFAAuthenticateResponse
		allowedReusedMFA          bool
		contextGetter             func() context.Context
		wantErrContains           string
		wantAdminActionAuthorized bool
	}{
		{
			name: "NOK local user no mfa",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username: localUser.GetName(),
					Groups:   localUser.GetRoles(),
				},
			},
			wantAdminActionAuthorized: false,
		}, {
			name: "NOK local user mfa verified cert",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username:    localUser.GetName(),
					Groups:      localUser.GetRoles(),
					MFAVerified: "mfa-verified-test",
				},
			},
			wantAdminActionAuthorized: false,
		}, {
			name: "NOK local user mfa verified private key policy",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username:         localUser.GetName(),
					Groups:           localUser.GetRoles(),
					PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
				},
			},
			wantAdminActionAuthorized: false,
		}, {
			// edge case for the admin role check.
			name: "NOK local user with host-like username",
			user: authz.LocalUser{
				Username: userWithHostName.GetName(),
				Identity: tlsca.Identity{
					Username: userWithHostName.GetName(),
					Groups:   userWithHostName.GetRoles(),
				},
			},
			wantAdminActionAuthorized: false,
		}, {
			name: "NOK local user invalid mfa",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username: localUser.GetName(),
					Groups:   localUser.GetRoles(),
				},
			},
			withMFA:                   invalidMFA,
			wantErrContains:           "access denied",
			wantAdminActionAuthorized: true,
		}, {
			name: "NOK local user reused mfa with reuse not allowed",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username: localUser.GetName(),
					Groups:   localUser.GetRoles(),
				},
			},
			withMFA:                   validMFAWithReuse,
			wantAdminActionAuthorized: false,
		}, {
			name: "OK local user valid mfa",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username: localUser.GetName(),
					Groups:   localUser.GetRoles(),
				},
			},
			withMFA:                   validMFA,
			wantAdminActionAuthorized: true,
		}, {
			name: "OK local user reused mfa with reuse allowed",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username: localUser.GetName(),
					Groups:   localUser.GetRoles(),
				},
			},
			withMFA:                   validMFAWithReuse,
			allowedReusedMFA:          true,
			wantAdminActionAuthorized: true,
		}, {
			name: "OK admin",
			user: authz.BuiltinRole{
				Role:     types.RoleAdmin,
				Username: hostFQDN(uuid.NewString(), clusterName),
			},
			wantAdminActionAuthorized: true,
		}, {
			name: "OK bot",
			user: authz.LocalUser{
				Username: bot.GetName(),
				Identity: tlsca.Identity{
					Username: bot.GetName(),
					Groups:   bot.GetRoles(),
				},
			},
			wantAdminActionAuthorized: true,
		}, {
			name: "OK admin impersonating local user",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username:     localUser.GetName(),
					Groups:       localUser.GetRoles(),
					Impersonator: hostFQDN(uuid.NewString(), clusterName),
				},
			},
			wantAdminActionAuthorized: true,
		}, {
			name: "OK bot impersonating local user",
			user: authz.LocalUser{
				Username: localUser.GetName(),
				Identity: tlsca.Identity{
					Username:     localUser.GetName(),
					Groups:       localUser.GetRoles(),
					Impersonator: bot.GetName(),
				},
			},
			wantAdminActionAuthorized: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.withMFA != nil {
				encodedMFAResp, err := mfa.EncodeMFAChallengeResponseCredentials(tt.withMFA)
				require.NoError(t, err)
				md := metadata.MD(map[string][]string{
					mfa.ResponseMetadataKey: {encodedMFAResp},
				})
				ctx = metadata.NewIncomingContext(ctx, md)
			}
			userCtx := authz.ContextWithUser(ctx, tt.user)
			authCtx, err := authorizer.Authorize(userCtx)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains, "Expected matching Authorize error")
				return
			}
			require.NoError(t, err)

			var authAdminActionErr error
			if tt.allowedReusedMFA {
				authAdminActionErr = authCtx.AuthorizeAdminActionAllowReusedMFA()
			} else {
				authAdminActionErr = authCtx.AuthorizeAdminAction()
			}

			if tt.wantAdminActionAuthorized {
				require.NoError(t, authAdminActionErr)
			} else {
				require.ErrorIs(t, authAdminActionErr, &mfa.ErrAdminActionMFARequired)
			}
		})
	}
}

func TestContext_GetAccessState(t *testing.T) {
	localCtx := authz.Context{
		User:    &fakeCtxUser{}, // makes no difference in the outcomes.
		Checker: &fakeCtxChecker{},
		Identity: authz.LocalUser{Identity: tlsca.Identity{
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
		createAuthCtx func() *authz.Context
		authSpec      *types.AuthPreferenceSpecV2 // defaults to defaultSpec
		want          services.AccessState
	}{
		{
			name:          "local user",
			createAuthCtx: func() *authz.Context { return &localCtx },
			want: services.AccessState{
				EnableDeviceVerification: true, // default when acquired from auth.Context
			},
		},
		{
			name: "builtin role",
			createAuthCtx: func() *authz.Context {
				ctx := localCtx
				ctx.Identity = authz.BuiltinRole{}
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
			createAuthCtx: func() *authz.Context {
				ctx := localCtx
				ctx.Checker = &fakeCtxChecker{state: services.AccessState{
					MFARequired: services.MFARequiredAlways,
				}}
				localUser := ctx.Identity.(authz.LocalUser)
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
			createAuthCtx: func() *authz.Context {
				ctx := localCtx
				localUser := ctx.Identity.(authz.LocalUser)
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
			createAuthCtx: func() *authz.Context {
				ctx := localCtx
				localUser := ctx.Identity.(authz.LocalUser)
				localUser.Identity.DeviceExtensions = deviceExt
				ctx.Identity = localUser

				authz.DisableContextDeviceRoleMode(&ctx)
				return &ctx
			},
			want: services.AccessState{
				EnableDeviceVerification: false, // copied from Context
				DeviceVerified:           true,  // Identity extensions
			},
		},
		{
			name: "bot user",
			createAuthCtx: func() *authz.Context {
				ctx := localCtx
				localUser := ctx.Identity.(authz.LocalUser)
				localUser.Identity.BotName = "wall-e"
				ctx.Identity = localUser
				return &ctx
			},
			want: services.AccessState{
				EnableDeviceVerification: true,
				IsBot:                    true,
			},
		},
	}
	for _, test := range tests {
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
			wantErr:    authz.ErrIPPinningMissing.Error(),
		},
		{
			desc:       "Pinned IP doesn't match",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.2",
			pinIP:      true,
			wantErr:    authz.ErrIPPinningMismatch.Error(),
		},
		{
			desc:       "Role doesn't require IP pinning now, but old certificate still pinned",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.2",
			pinIP:      false,
			wantErr:    authz.ErrIPPinningMismatch.Error(),
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
			wantErr:    authz.ErrIPPinningNotAllowed.Error(),
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
			ctx = authz.ContextWithClientSrcAddr(ctx, utils.MustParseAddr(tt.clientAddr))
		}
		identity := tlsca.Identity{PinnedIP: tt.pinnedIP}

		err := authz.CheckIPPinning(ctx, identity, tt.pinIP, nil)

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
			rs, err := authz.RoleSetForBuiltinRoles(test.clusterName, test.recConfig, test.roles...)
			require.NoError(t, err, "RoleSetForBuiltinRoles failed")
			assert.NotEmpty(t, rs, "RoleSetForBuiltinRoles returned a nil RoleSet")
			test.assertRoleSet(t, rs)
		})
	}
}

func TestIsUserFunctions(t *testing.T) {
	localIdentity := authz.Context{
		Identity:         authz.LocalUser{},
		UnmappedIdentity: authz.LocalUser{},
	}
	remoteIdentity := authz.Context{
		Identity:         authz.RemoteUser{},
		UnmappedIdentity: authz.RemoteUser{},
	}
	systemIdentity := authz.Context{
		Identity:         authz.BuiltinRole{Role: types.RoleProxy},
		UnmappedIdentity: authz.BuiltinRole{Role: types.RoleProxy},
	}

	tests := []struct {
		funcName, scenario string
		isUserFunc         func(authz.Context) bool
		authCtx            authz.Context
		want               bool
	}{
		{
			funcName:   "IsLocalUser",
			scenario:   "local user",
			isUserFunc: authz.IsLocalUser,
			authCtx:    localIdentity,
			want:       true,
		},
		{
			funcName:   "IsLocalUser",
			scenario:   "remote user",
			isUserFunc: authz.IsLocalUser,
			authCtx:    remoteIdentity,
		},
		{
			funcName:   "IsLocalUser",
			scenario:   "system user",
			isUserFunc: authz.IsLocalUser,
			authCtx:    systemIdentity,
		},
		{
			funcName:   "IsRemoteUser",
			scenario:   "local user",
			isUserFunc: authz.IsRemoteUser,
			authCtx:    localIdentity,
		},
		{
			funcName:   "IsRemoteUser",
			scenario:   "remote user",
			isUserFunc: authz.IsRemoteUser,
			authCtx:    remoteIdentity,
			want:       true,
		},
		{
			funcName:   "IsRemoteUser",
			scenario:   "system user",
			isUserFunc: authz.IsRemoteUser,
			authCtx:    systemIdentity,
		},

		{
			funcName:   "IsLocalOrRemoteUser",
			scenario:   "local user",
			isUserFunc: authz.IsLocalOrRemoteUser,
			authCtx:    localIdentity,
			want:       true,
		},
		{
			funcName:   "IsLocalOrRemoteUser",
			scenario:   "remote user",
			isUserFunc: authz.IsLocalOrRemoteUser,
			authCtx:    remoteIdentity,
			want:       true,
		},
		{
			funcName:   "IsLocalOrRemoteUser",
			scenario:   "system user",
			isUserFunc: authz.IsLocalOrRemoteUser,
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

func TestConnectionMetadata(t *testing.T) {
	for name, test := range map[string]struct {
		ctx                        context.Context
		expectedConnectionMetadata apievents.ConnectionMetadata
	}{
		"with client address": {
			ctx:                        authz.ContextWithClientSrcAddr(context.Background(), &net.TCPAddr{IP: net.IPv4(10, 255, 0, 0), Port: 1234}),
			expectedConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: "10.255.0.0:1234"},
		},
		"empty client address": {
			ctx:                        context.Background(),
			expectedConnectionMetadata: apievents.ConnectionMetadata{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, cmp.Diff(test.expectedConnectionMetadata, authz.ConnectionMetadata(test.ctx)))
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

func (c *fakeCtxChecker) GetAccessState(_ readonly.AuthPreference) services.AccessState {
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

func newTestResources(t *testing.T) (*testClient, *services.LockWatcher, authz.Authorizer) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, backend.Close())
	})

	clusterConfig, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	caSvc := local.NewCAService(backend)
	accessSvc := local.NewAccessService(backend)
	identitySvc, err := local.NewTestIdentityService(backend)
	require.NoError(t, err)
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
	_, err = client.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	err = client.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig())
	require.NoError(t, err)
	_, err = client.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = client.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	lockSvc := local.NewAccessService(backend)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
		LockGetter: lockSvc,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: client,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	return client, lockWatcher, authorizer
}

func resourceDiff(res1, res2 types.Resource) string {
	return cmp.Diff(res1, res2,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.EquateEmpty())
}
