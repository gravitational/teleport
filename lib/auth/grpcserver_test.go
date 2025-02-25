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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	prom_client_model "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	otlpresourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	otlptracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/server/installer"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMFADeviceManagement(t *testing.T) {
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := testServer.Clock().(*clockwork.FakeClock)
	ctx := context.Background()

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: constants.Local,
		SecondFactors: []types.SecondFactorType{
			types.SecondFactorType_SECOND_FACTOR_TYPE_OTP,
			types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
			types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
		},
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	const webOrigin = "https://localhost" // matches RPID above
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(authServer, "mfa-user", []string{"role"}, nil)
	require.NoError(t, err)
	userClient, err := testServer.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// No MFA devices should exist for a new user.
	resp, err := userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Devices)

	// Add one device of each kind
	devs := addOneOfEachMFADevice(t, userClient, clock, webOrigin)

	// Run scenarios beyond adding one of each device, both happy and failures.
	webKey2, err := mocku2f.Create()
	require.NoError(t, err)
	webKey2.PreferRPID = true
	const webDev2Name = "webauthn2"
	const pwdlessDevName = "pwdless"

	addTests := []struct {
		desc string
		opts mfaAddTestOpts
	}{
		{
			desc: "fail TOTP auth challenge",
			opts: mfaAddTestOpts{
				deviceName: "fail-dev",
				deviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)

					// Respond to challenge using an unregistered TOTP device,
					// which should fail the auth challenge.
					badDev, err := totp.Generate(totp.GenerateOpts{Issuer: "Teleport", AccountName: user.GetName()})
					require.NoError(t, err)
					code, err := totp.GenerateCode(badDev.Secret(), clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkAuthErr: require.Error,
			},
		},
		{
			desc: "fail a TOTP registration challenge",
			opts: mfaAddTestOpts{
				deviceName:   "fail-dev",
				deviceType:   proto.DeviceType_DEVICE_TYPE_TOTP,
				authHandler:  devs.totpAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					totpRegisterChallenge := req.GetTOTP()
					require.NotEmpty(t, totpRegisterChallenge)
					require.Equal(t, totpRegisterChallenge.Algorithm, otp.AlgorithmSHA1.String())
					// Use the wrong secret for registration, causing server
					// validation to fail.
					code, err := totp.GenerateCodeCustom(base32.StdEncoding.EncodeToString([]byte("wrong-secret")), clock.Now(), totp.ValidateOpts{
						Period:    uint(totpRegisterChallenge.PeriodSeconds),
						Digits:    otp.Digits(totpRegisterChallenge.Digits),
						Algorithm: otp.AlgorithmSHA1,
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{
							Code: code,
						}},
					}
				},
				checkRegisterErr: require.Error,
			},
		},
		{
			desc: "add a second webauthn device",
			opts: mfaAddTestOpts{
				deviceName:   webDev2Name,
				deviceType:   proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				authHandler:  devs.webAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					ccr, err := webKey2.SignCredentialCreation(webOrigin, wantypes.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: require.NoError,
			},
		},
		{
			desc: "fail a webauthn auth challenge",
			opts: mfaAddTestOpts{
				deviceName: "webauthn-1512000",
				deviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				authHandler: func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, challenge.WebauthnChallenge) // webauthn enabled

					// Sign challenge with an unknown device.
					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true
					key.IgnoreAllowedCredentials = true
					resp, err := key.SignAssertion(webOrigin, wantypes.CredentialAssertionFromProto(challenge.WebauthnChallenge))
					require.NoError(t, err)
					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: wantypes.CredentialAssertionResponseToProto(resp),
						},
					}
				},
				checkAuthErr: func(t require.TestingT, err error, i ...interface{}) {
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				},
			},
		},
		{
			desc: "fail a webauthn registration challenge",
			opts: mfaAddTestOpts{
				deviceName:   "webauthn-1512000",
				deviceType:   proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				authHandler:  devs.webAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					require.NotNil(t, challenge.GetWebauthn())

					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true

					ccr, err := key.SignCredentialCreation(
						"http://badorigin.com" /* origin */, wantypes.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)
					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: func(t require.TestingT, err error, i ...interface{}) {
					require.Error(t, err)
					require.True(t, trace.IsBadParameter(err))
				},
			},
		},
		{
			desc: "add passwordless device",
			opts: mfaAddTestOpts{
				deviceName:   pwdlessDevName,
				deviceType:   proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				deviceUsage:  proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
				authHandler:  devs.webAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					require.NotNil(t, challenge.GetWebauthn(), "WebAuthn challenge cannot be nil")

					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true
					key.SetPasswordless()

					ccr, err := key.SignCredentialCreation(webOrigin, wantypes.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: require.NoError,
				assertRegisteredDev: func(t *testing.T, dev *types.MFADevice) {
					// Do a few simple device checks - lib/auth/webauthn goes in depth.
					require.NotNil(t, dev.GetWebauthn(), "WebAuthnDevice cannot be nil")
					require.True(t, true, dev.GetWebauthn().ResidentKey, "ResidentKey should be set to true")
				},
			},
		},
	}
	for _, test := range addTests {
		t.Run(test.desc, func(t *testing.T) {
			testAddMFADevice(ctx, t, userClient, test.opts)
		})
	}

	// Register an extra device to test allow deletion of other devices and test that
	// the last device cannot be deleted.
	const lastDeviceName = "lastDevice"
	lastDevice, err := RegisterTestDevice(ctx, userClient, lastDeviceName, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, devs.WebDev)
	require.NoError(t, err, "RegisterTestDevice failed")

	// Also add a password so we can testing add last non-passkey MFA device. Testing the
	// deletion of the last passkey is handled in TestDeletingLastPasswordlessDevice below.
	err = authServer.UpsertPassword(user.GetName(), []byte("living on the edge"))
	require.NoError(t, err, "UpsertPassword")

	// Since this device won't be deleted, we can use it to solve webauthn
	// challenges throughout the tests below.
	lastDeviceWebAuthnHandler := func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
		require.NotNil(t, challenge.WebauthnChallenge, "nil Webauthn challenge")
		mfaResp, err := lastDevice.SolveAuthn(challenge)
		require.NoError(t, err, "SolveAuthn")
		return mfaResp
	}

	// Check that all new devices are registered.
	resp, err = userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	deviceNames := make([]string, 0, len(resp.Devices))
	deviceIDs := make(map[string]string)
	for _, dev := range resp.Devices {
		deviceNames = append(deviceNames, dev.GetName())
		deviceIDs[dev.GetName()] = dev.Id
	}
	sort.Strings(deviceNames)
	require.Equal(t, []string{lastDeviceName, pwdlessDevName, devs.TOTPName, devs.WebName, webDev2Name}, deviceNames)

	// Delete several of the MFA devices.
	deleteTests := []struct {
		desc string
		opts mfaDeleteTestOpts
	}{
		{
			desc: "fail to delete an unknown device",
			opts: mfaDeleteTestOpts{
				deviceName:  "unknown-dev",
				authHandler: lastDeviceWebAuthnHandler,
				checkErr:    require.Error,
			},
		},
		{
			desc: "fail a TOTP auth challenge",
			opts: mfaDeleteTestOpts{
				deviceName: devs.TOTPName,
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)

					// Respond to challenge using an unregistered TOTP device,
					// which should fail the auth challenge.
					badDev, err := totp.Generate(totp.GenerateOpts{Issuer: "Teleport", AccountName: user.GetName()})
					require.NoError(t, err)
					code, err := totp.GenerateCode(badDev.Secret(), clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "fail a webauthn auth challenge",
			opts: mfaDeleteTestOpts{
				deviceName: devs.WebName,
				authHandler: func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, challenge.WebauthnChallenge)

					// Sign challenge with an unknown device.
					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true
					key.IgnoreAllowedCredentials = true
					resp, err := key.SignAssertion(webOrigin, wantypes.CredentialAssertionFromProto(challenge.WebauthnChallenge))
					require.NoError(t, err)
					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: wantypes.CredentialAssertionResponseToProto(resp),
						},
					}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "delete TOTP device by name",
			opts: mfaDeleteTestOpts{
				deviceName:  devs.TOTPName,
				authHandler: lastDeviceWebAuthnHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "delete webauthn device by name",
			opts: mfaDeleteTestOpts{
				deviceName:  devs.WebName,
				authHandler: lastDeviceWebAuthnHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "delete passwordless device by name",
			opts: mfaDeleteTestOpts{
				deviceName:  pwdlessDevName,
				authHandler: lastDeviceWebAuthnHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "delete webauthn device by ID",
			opts: mfaDeleteTestOpts{
				deviceName:  deviceIDs[webDev2Name],
				authHandler: lastDeviceWebAuthnHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "fail to delete last device",
			opts: mfaDeleteTestOpts{
				deviceName:  lastDeviceName,
				authHandler: lastDeviceWebAuthnHandler,
				checkErr: func(t require.TestingT, err error, _ ...any) {
					require.ErrorContains(t,
						err,
						"cannot delete the last MFA device for this user",
						"Unexpected error deleting last MFA device",
					)
				},
			},
		},
	}
	for _, test := range deleteTests {
		t.Run(test.desc, func(t *testing.T) {
			testDeleteMFADevice(ctx, t, userClient, test.opts)
		})
	}

	// Check no remaining devices, apart from the additional device that we can't delete.
	resp, err = userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Equal(t, lastDeviceName, resp.Devices[0].GetName())
}

func TestMFADeviceManagement_SSO(t *testing.T) {
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	ctx := context.Background()

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: constants.Local,
		SecondFactors: []types.SecondFactorType{
			types.SecondFactorType_SECOND_FACTOR_TYPE_OTP,
			types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
			types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
		},
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(authServer, "mfa-user", []string{"role"}, nil)
	require.NoError(t, err)
	userClient, err := testServer.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Create an auth connector.
	samlConnector, err := types.NewSAMLConnector("saml", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "http://localhost:65535/acs", // not called
		Issuer:                   "test",
		SSO:                      "https://localhost:65535/sso", // not called
		AttributesToRoles: []types.AttributeMapping{
			// not used. can be any name, value but role must exist
			{Name: "groups", Value: "admin", Roles: user.GetRoles()},
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertSAMLConnector(ctx, samlConnector)
	require.NoError(t, err)

	// Convert the user to an SSO user for this auth connector.
	userCreatedAt := authServer.clock.Now()
	user.SetCreatedBy(types.CreatedBy{
		Time: userCreatedAt,
		Connector: &types.ConnectorRef{
			ID:   samlConnector.GetKind(),
			Type: samlConnector.GetName(),
		},
	})
	_, err = authServer.UpsertUser(ctx, user)
	require.NoError(t, err)

	// No MFA devices should exist for the user.
	resp, err := userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Devices)

	// prepare a passwordless device.
	passkeyName := "passkey"
	passkey, err := RegisterTestDevice(ctx, userClient, passkeyName, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil, WithPasswordless())
	require.NoError(t, err, "RegisterTestDevice")

	passkeyWebAuthnHandler := func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
		require.NotNil(t, challenge.WebauthnChallenge, "nil Webauthn challenge")
		mfaResp, err := passkey.SolveAuthn(challenge)
		require.NoError(t, err, "SolveAuthn")
		return mfaResp
	}

	resp, err = userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Devices, 1)
	webauthnDevice := resp.Devices[0]

	// Update the auth connector to enable SSO MFA.
	samlConnector.SetMFASettings(&types.SAMLConnectorMFASettings{
		Enabled: true,
		Issuer:  "test",
		Sso:     "https://localhost:65535/sso", // not called
	})
	require.NoError(t, err)
	_, err = authServer.UpsertSAMLConnector(ctx, samlConnector)
	require.NoError(t, err)

	// Ephemeral sso device should show up in the list now. It can't be deleted.
	resp, err = userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Devices, 2)

	expectSSODev, err := types.NewMFADevice(samlConnector.GetDisplay(), samlConnector.GetName(), userCreatedAt, &types.MFADevice_Sso{
		Sso: &types.SSOMFADevice{
			ConnectorId:   samlConnector.GetName(),
			ConnectorType: samlConnector.GetKind(),
			DisplayName:   samlConnector.GetDisplay(),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, []*types.MFADevice{webauthnDevice, expectSSODev}, resp.Devices)

	testDeleteMFADevice(ctx, t, userClient, mfaDeleteTestOpts{
		deviceName:  "saml",
		authHandler: passkeyWebAuthnHandler,
		checkErr: func(t require.TestingT, err error, _ ...interface{}) {
			assert.ErrorAs(t, err, new(*trace.BadParameterError))
			assert.ErrorContains(t, err, "cannot delete ephemeral SSO MFA device")
		}},
	)

	// Last non-SSO, passwordless device can be deleted now.
	testDeleteMFADevice(ctx, t, userClient, mfaDeleteTestOpts{
		deviceName:  passkeyName,
		authHandler: passkeyWebAuthnHandler,
		checkErr:    require.NoError,
	})
}

func TestDeletingLastPasswordlessDevice(t *testing.T) {
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := testServer.Clock().(*clockwork.FakeClock)
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice)
		checkErr require.ErrorAssertionFunc
	}{
		{
			name:  "NOK no other MFA device",
			setup: func(*testing.T, string, *authclient.Client, *TestDevice) {},
			checkErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t,
					err,
					"cannot delete the last MFA device for this user",
					"Unexpected error deleting last passwordless device",
				)
			},
		},
		{
			name: "OK extra passwordless device",
			setup: func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				_, err := RegisterTestDevice(ctx, userClient, "another-passkey", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, pwdlessDev, WithPasswordless())
				require.NoError(t, err, "RegisterTestDevice failed")
			},
			checkErr: require.NoError,
		},
		{
			name: "OK password set with other WebAuthn device",
			setup: func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				err := authServer.UpsertPassword(username, []byte("living on the edge"))
				require.NoError(t, err, "UpsertPassword")
				_, err = RegisterTestDevice(
					ctx, userClient, "another-dev", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, pwdlessDev)
				require.NoError(t, err, "RegisterTestDevice")
			},
			checkErr: require.NoError,
		},
		{
			name: "OK password set with other TOTP device",
			setup: func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				err := authServer.UpsertPassword(username, []byte("living on the edge"))
				require.NoError(t, err, "UpsertPassword")
				_, err = RegisterTestDevice(
					ctx, userClient, "another-dev", proto.DeviceType_DEVICE_TYPE_TOTP, pwdlessDev, WithTestDeviceClock(clock))
				require.NoError(t, err, "RegisterTestDevice")
			},
			checkErr: require.NoError,
		},
		{
			name: "OK SSO user with other device",
			setup: func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				user, err := authServer.GetUser(ctx, username, false)
				require.NoError(t, err, "GetUser")
				user.SetCreatedBy(types.CreatedBy{
					Connector: &types.ConnectorRef{},
				})
				_, err = authServer.UpsertUser(ctx, user)
				require.NoError(t, err, "UpsertUser")
				_, err = RegisterTestDevice(
					ctx, userClient, "another-dev", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, pwdlessDev)
				require.NoError(t, err, "RegisterTestDevice")
			},
			checkErr: require.NoError,
		},
		{
			name: "NOK password set but no other MFAs",
			setup: func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				err := authServer.UpsertPassword(username, []byte("living on the edge"))
				require.NoError(t, err, "UpsertPassword")
			},
			checkErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t,
					err,
					"cannot delete the last MFA device for this user",
					"Unexpected error deleting last passwordless device",
				)
			},
		},
		{
			name: "NOK other MFAs, but no password set",
			setup: func(t *testing.T, username string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				_, err := RegisterTestDevice(
					ctx, userClient, "another-dev", proto.DeviceType_DEVICE_TYPE_TOTP, pwdlessDev, WithTestDeviceClock(clock))
				require.NoError(t, err, "RegisterTestDevice")
			},
			checkErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t,
					err,
					"cannot delete last passwordless credential for user",
					"Unexpected error deleting last passwordless device",
				)
			},
		},
		{
			name: "NOK other MFAs, but no password set, passwordless is off",
			setup: func(t *testing.T, _ string, userClient *authclient.Client, pwdlessDev *TestDevice) {
				// Register a non-passwordless device without adding a password.
				_, err := RegisterTestDevice(ctx, userClient, "another-dev", proto.DeviceType_DEVICE_TYPE_TOTP, pwdlessDev, WithTestDeviceClock(clock))
				require.NoError(t, err, "RegisterTestDevice")

				authPref, err := authServer.GetAuthPreference(ctx)
				require.NoError(t, err, "GetAuthPreference")

				// Turn off passwordless authentication.
				authPref.SetAllowPasswordless(false)
				_, err = authServer.UpsertAuthPreference(ctx, authPref)
				require.NoError(t, err, "UpsertAuthPreference")
			},
			checkErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t,
					err,
					"cannot delete last passwordless credential for user",
					"Unexpected error deleting last passwordless device",
				)
			},
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Enable MFA support.
			authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				Type: constants.Local,
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_OTP,
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			})
			require.NoError(t, err)
			_, err = authServer.UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			// Create a fake user.
			username := fmt.Sprintf("mfa-user-%d", i)
			user, _, err := CreateUserAndRole(authServer, username, []string{"role"}, nil)
			require.NoError(t, err)
			userClient, err := testServer.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			// No MFA devices should exist for a new user.
			resp, err := userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
			require.NoError(t, err)
			require.Empty(t, resp.Devices)

			// Add the passwordless device to be deleted.
			pwdlessDevName := "pwdless-dev"
			pwdlessDev, err := RegisterTestDevice(
				ctx, userClient, pwdlessDevName, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil, WithPasswordless())
			require.NoError(t, err)

			// Case-specific setup.
			test.setup(t, username, userClient, pwdlessDev)

			// Delete the last passwordless device.
			testDeleteMFADevice(ctx, t, userClient, mfaDeleteTestOpts{
				deviceName: pwdlessDevName,
				authHandler: func(t *testing.T, ch *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, ch.WebauthnChallenge, "nil Webauthn challenge")

					mfaResp, err := pwdlessDev.SolveAuthn(ch)
					require.NoError(t, err, "SolveAuthn")

					return mfaResp
				},
				checkErr: test.checkErr,
			})
		})
	}
}

type mfaDevices struct {
	clock     clockwork.Clock
	webOrigin string

	TOTPName string
	TOTPDev  *TestDevice

	WebName string
	WebDev  *TestDevice
}

func (d *mfaDevices) totpAuthHandler(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
	require.NotNil(t, challenge.TOTP, "nil TOTP challenge")

	if c, ok := d.clock.(*clockwork.FakeClock); ok {
		c.Advance(30 * time.Second)
	}

	mfaResp, err := d.TOTPDev.SolveAuthn(challenge)
	require.NoError(t, err, "SolveAuthn")

	return mfaResp
}

func (d *mfaDevices) webAuthHandler(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
	require.NotNil(t, challenge.WebauthnChallenge, "nil Webauthn challenge")

	mfaResp, err := d.WebDev.SolveAuthn(challenge)
	require.NoError(t, err, "SolveAuthn")

	return mfaResp
}

func addOneOfEachMFADevice(t *testing.T, userClient *authclient.Client, clock clockwork.Clock, origin string) mfaDevices {
	const totpName = "totp-dev"
	const webName = "webauthn-dev"

	ctx := context.Background()

	totpDev, err := RegisterTestDevice(
		ctx, userClient, totpName, proto.DeviceType_DEVICE_TYPE_TOTP, nil /* authenticator */, WithTestDeviceClock(clock))
	require.NoError(t, err, "RegisterTestDevice(totp)")

	webDev, err := RegisterTestDevice(
		ctx, userClient, webName, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, totpDev /* authenticator */)
	require.NoError(t, err, "RegisterTestDevice(totp)")

	return mfaDevices{
		clock:     clock,
		webOrigin: origin,
		TOTPName:  totpName,
		WebName:   webName,
		TOTPDev:   totpDev,
		WebDev:    webDev,
	}
}

type mfaAddTestOpts struct {
	deviceName  string
	deviceType  proto.DeviceType
	deviceUsage proto.DeviceUsage

	authHandler         func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkAuthErr        require.ErrorAssertionFunc
	registerHandler     func(*testing.T, *proto.MFARegisterChallenge) *proto.MFARegisterResponse
	checkRegisterErr    require.ErrorAssertionFunc
	assertRegisteredDev func(*testing.T, *types.MFADevice)
}

func testAddMFADevice(ctx context.Context, t *testing.T, authClient *authclient.Client, opts mfaAddTestOpts) {
	authChal, err := authClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	require.NoError(t, err, "CreateAuthenticateChallenge")
	authnSolved := opts.authHandler(t, authChal)

	registerChal, err := authClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: authnSolved,
		DeviceType:          opts.deviceType,
		DeviceUsage:         opts.deviceUsage,
	})
	opts.checkAuthErr(t, err)
	if err != nil {
		return
	}
	registerSolved := opts.registerHandler(t, registerChal)

	addResp, err := authClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
		NewDeviceName:  opts.deviceName,
		NewMFAResponse: registerSolved,
		DeviceUsage:    opts.deviceUsage,
	})
	opts.checkRegisterErr(t, err)
	switch {
	case err != nil:
		return
	case opts.assertRegisteredDev != nil:
		opts.assertRegisteredDev(t, addResp.Device)
	}
}

type mfaDeleteTestOpts struct {
	deviceName  string
	authHandler func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkErr    require.ErrorAssertionFunc
}

func testDeleteMFADevice(ctx context.Context, t *testing.T, authClient *authclient.Client, opts mfaDeleteTestOpts) {
	// Issue and solve authn challenge.
	authnChal, err := authClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	require.NoError(t, err, "CreateAuthenticateChallenge")
	authnSolved := opts.authHandler(t, authnChal)

	// Attempt deletion.
	opts.checkErr(t,
		authClient.DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
			DeviceName:          opts.deviceName,
			ExistingMFAResponse: authnSolved,
		}))
}

func TestCreateAppSession_deviceExtensions(t *testing.T) {
	ctx := context.Background()
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	// Create an user for testing.
	user, _, err := CreateUserAndRole(authServer, "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")

	// Register an application.
	app, err := types.NewAppV3(
		types.Metadata{
			Name: "llamaapp",
		}, types.AppSpecV3{
			URI:        "http://localhost:8080",
			PublicAddr: "llamaapp.example.com",
		})
	require.NoError(t, err, "NewAppV3 failed")
	appServer, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err, "NewAppServerV3FromApp failed")
	_, err = authServer.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err, "UpsertApplicationServer failed")

	wantExtensions := &tlsca.DeviceExtensions{
		DeviceID:     "device1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	tests := []struct {
		name       string
		modifyUser func(u *TestIdentity)
		assertCert func(t *testing.T, cert *x509.Certificate)
	}{
		{
			name: "no device extensions",
			// Absence of errors is enough here, this is mostly to make sure the base
			// scenario works.
		},
		{
			name: "user with device extensions",
			modifyUser: func(u *TestIdentity) {
				lu := u.I.(authz.LocalUser)
				lu.Identity.DeviceExtensions = *wantExtensions
				u.I = lu
			},
			assertCert: func(t *testing.T, cert *x509.Certificate) {
				gotIdentity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
				require.NoError(t, err, "FromSubject failed")

				if diff := cmp.Diff(*wantExtensions, gotIdentity.DeviceExtensions, protocmp.Transform()); diff != "" {
					t.Errorf("DeviceExtensions mismatch (-want +got)\n%s", diff)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := TestUser(user.GetName())
			if test.modifyUser != nil {
				test.modifyUser(&u)
			}

			userClient, err := testServer.NewClient(u)
			require.NoError(t, err, "NewClient failed")

			session, err := userClient.CreateAppSession(ctx, &proto.CreateAppSessionRequest{
				Username:    user.GetName(),
				PublicAddr:  app.GetPublicAddr(),
				ClusterName: testServer.ClusterName(),
			})
			require.NoError(t, err, "CreateAppSession failed")

			block, _ := pem.Decode(session.GetTLSCert())
			require.NotNil(t, block, "Decode failed")
			gotCert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err, "ParserCertificate failed")

			if test.assertCert != nil {
				test.assertCert(t, gotCert)
			}
		})
	}
}

func TestGenerateUserCerts_deviceExtensions(t *testing.T) {
	ctx := context.Background()
	testServer := newTestTLSServer(t)

	// Create an user for testing.
	user, _, err := CreateUserAndRole(testServer.Auth(), "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err, "GenerateKeyWithAlgorithm failed")
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err, "MarshalPublicKey failed")

	wantExtensions := &tlsca.DeviceExtensions{
		DeviceID:     "device1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	tests := []struct {
		name       string
		modifyUser func(u *TestIdentity)
		assertCert func(t *testing.T, cert *x509.Certificate)
	}{
		{
			name: "no device extensions",
			// Absence of errors is enough here, this is mostly to make sure the base
			// scenario works.
		},
		{
			name: "user with device extensions",
			modifyUser: func(u *TestIdentity) {
				lu := u.I.(authz.LocalUser)
				lu.Identity.DeviceExtensions = *wantExtensions
				u.I = lu
			},
			assertCert: func(t *testing.T, cert *x509.Certificate) {
				gotIdentity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
				require.NoError(t, err, "FromSubject failed")

				if diff := cmp.Diff(*wantExtensions, gotIdentity.DeviceExtensions, protocmp.Transform()); diff != "" {
					t.Errorf("DeviceExtensions mismatch (-want +got)\n%s", diff)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := TestUser(user.GetName())
			if test.modifyUser != nil {
				test.modifyUser(&u)
			}

			userClient, err := testServer.NewClient(u)
			require.NoError(t, err, "NewClient failed")

			resp, err := userClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
				TLSPublicKey: publicKeyPEM,
				Username:     user.GetName(),
				Expires:      testServer.Clock().Now().Add(1 * time.Hour),
			})
			require.NoError(t, err, "GenerateUserCerts failed")

			block, _ := pem.Decode(resp.TLS)
			require.NotNil(t, block, "Decode failed")
			gotCert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err, "ParserCertificate failed")

			if test.assertCert != nil {
				test.assertCert(t, gotCert)
			}
		})
	}
}

func TestGenerateUserCerts_deviceAuthz(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise, // required for Device Trust.
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.App: {Enabled: true},
			},
		},
	})

	testServer := newTestTLSServer(t)

	ctx := context.Background()
	clock := testServer.Clock()
	clusterName := testServer.ClusterName()
	authServer := testServer.Auth()

	// Create a user for testing.
	user, role, err := CreateUserAndRole(testServer.Auth(), "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	username := user.GetName()

	// Make sure MFA is required for this user.
	roleOpt := role.GetOptions()
	roleOpt.RequireMFAType = types.RequireMFAType_SESSION
	role.SetOptions(roleOpt)

	_, err = authServer.UpsertRole(ctx, role)
	require.NoError(t, err)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "mynode",
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err = authServer.UpsertNode(ctx, node)
	require.NoError(t, err)

	// Create clients with and without device extensions.
	clientWithoutDevice, err := testServer.NewClient(TestUser(username))
	require.NoError(t, err, "NewClient failed")

	clientWithDevice, err := testServer.NewClient(
		TestUserWithDeviceExtensions(username, tlsca.DeviceExtensions{
			DeviceID:     "deviceid1",
			AssetTag:     "assettag1",
			CredentialID: "credentialid1",
		}))
	require.NoError(t, err, "NewClient failed")

	// updateAuthPref is a helper used throughout the test.
	updateAuthPref := func(t *testing.T, modify func(ap types.AuthPreference)) {
		authPref, err := authServer.GetAuthPreference(ctx)
		require.NoError(t, err, "GetAuthPreference failed")

		modify(authPref)

		_, err = authServer.UpsertAuthPreference(ctx, authPref)
		require.NoError(t, err, "UpsertAuthPreference failed")
	}

	// Register MFA devices for the user.
	// Required to issue certificates with MFA.
	const rpID = "localhost"
	const origin = "https://" + rpID + ":3080" // matches RPID.
	updateAuthPref(t, func(authPref types.AuthPreference) {
		authPref.SetSecondFactors(
			types.SecondFactorType_SECOND_FACTOR_TYPE_OTP,
			types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
		)
		authPref.SetWebauthn(&types.Webauthn{
			RPID: "localhost",
		})
	})
	mfaDevices := addOneOfEachMFADevice(t, clientWithoutDevice, clock, origin)

	mfaPromptFunc := mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		return mfaDevices.WebDev.SolveAuthn(chal)
	})
	promptConstructor := func(...mfa.PromptOpt) mfa.Prompt { return mfaPromptFunc }
	clientWithoutDevice.SetMFAPromptConstructor(promptConstructor)
	clientWithDevice.SetMFAPromptConstructor(promptConstructor)

	// Create public keys for UserCertsRequest.
	_, sshPub, _, tlsPub := newSSHAndTLSKeyPairs(t)

	expires := clock.Now().Add(1 * time.Hour)
	sshReq := proto.UserCertsRequest{
		SSHPublicKey:   sshPub,
		Username:       username,
		Expires:        expires,
		RouteToCluster: clusterName,
		NodeName:       "mynode",
		Usage:          proto.UserCertsRequest_SSH,
		SSHLogin:       "llama",
	}
	appReq := proto.UserCertsRequest{
		TLSPublicKey:   tlsPub,
		Username:       username,
		Expires:        expires,
		RouteToCluster: clusterName,
		Usage:          proto.UserCertsRequest_App,
		RouteToApp: proto.RouteToApp{
			Name:        "hello",
			PublicAddr:  "hello.cluster.dev",
			ClusterName: clusterName,
		},
	}
	winReq := proto.UserCertsRequest{
		TLSPublicKey:   tlsPub,
		Username:       username,
		Expires:        expires,
		RouteToCluster: clusterName,
		Usage:          proto.UserCertsRequest_WindowsDesktop,
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: "mydesktop",
			Login:          username,
		},
	}

	assertSuccess := func(t *testing.T, err error) {
		assert.NoError(t, err, "GenerateUserCerts error mismatch")
	}
	assertAccessDenied := func(t *testing.T, err error) {
		assert.True(t, trace.IsAccessDenied(err), "GenerateUserCerts error mismatch, got=%v (%T), want trace.AccessDeniedError", err, err)
	}

	// generateCertsMFA is used to generate single-use, MFA-enabled certificates.
	generateCertsMFA := func(t *testing.T, client *authclient.Client, req proto.UserCertsRequest) (cert *proto.Certs, err error) {
		defer func() {
			// Translate gRPC to trace errors, as our clients do.
			err = trail.FromGRPC(err)
		}()

		authnChal, err := client.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
			},
		})
		if err != nil {
			return nil, err
		}

		req.MFAResponse = mfaDevices.webAuthHandler(t, authnChal)
		req.Purpose = proto.UserCertsRequest_CERT_PURPOSE_SINGLE_USE_CERTS
		return client.GenerateUserCerts(ctx, req)
	}

	tests := []struct {
		name               string
		clusterDeviceMode  string
		client             *authclient.Client
		req                proto.UserCertsRequest
		skipLoginCerts     bool // aka non-MFA issuance.
		skipSingleUseCerts bool // aka MFA/streaming issuance.
		assertErr          func(t *testing.T, err error)
	}{
		{
			name:              "mode=optional without extensions",
			clusterDeviceMode: constants.DeviceTrustModeOptional,
			client:            clientWithoutDevice,
			req:               sshReq,
			assertErr:         assertSuccess,
		},
		{
			name:              "mode=optional with extensions",
			clusterDeviceMode: constants.DeviceTrustModeOptional,
			client:            clientWithDevice,
			req:               sshReq,
			assertErr:         assertSuccess,
		},
		{
			name:              "nok: mode=required without extensions",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithoutDevice,
			req:               sshReq,
			assertErr:         assertAccessDenied,
		},
		{
			name:              "mode=required with extensions",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithDevice,
			req:               sshReq,
			assertErr:         assertSuccess,
		},
		{
			name:               "mode=required ignores App Access requests (non-MFA)",
			clusterDeviceMode:  constants.DeviceTrustModeRequired,
			client:             clientWithoutDevice,
			req:                appReq,
			skipSingleUseCerts: true,
			assertErr:          assertSuccess,
		},
		{
			name:              "mode=required ignores App Access requests",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithoutDevice,
			req:               appReq,
			skipLoginCerts:    true,
			assertErr:         assertSuccess,
		},
		{
			name:              "mode=required ignores Desktop Access requests",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithoutDevice,
			req:               winReq,
			assertErr:         assertSuccess,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updateAuthPref(t, func(ap types.AuthPreference) {
				ap.SetDeviceTrust(&types.DeviceTrust{
					Mode: test.clusterDeviceMode,
				})
			})

			if !test.skipLoginCerts {
				t.Run("login certs", func(t *testing.T) {
					_, err := test.client.GenerateUserCerts(ctx, test.req)
					test.assertErr(t, err)
				})
			}

			if !test.skipSingleUseCerts {
				t.Run("single-use certs", func(t *testing.T) {
					_, err := generateCertsMFA(t, test.client, test.req)
					test.assertErr(t, err)
				})
			}
		})
	}
}

// Test that device trust is required for a user registering their first MFA device.
func TestRegisterFirstDevice_deviceAuthz(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise, // required for Device Trust.
	})

	testServer := newTestTLSServer(t)

	ctx := context.Background()
	authServer := testServer.Auth()

	// Create a user for testing.
	user, _, err := CreateUserAndRole(testServer.Auth(), "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	username := user.GetName()

	// Create clients with and without device extensions.
	clientWithoutDevice, err := testServer.NewClient(TestUser(username))
	require.NoError(t, err, "NewClient failed")

	clientWithDevice, err := testServer.NewClient(
		TestUserWithDeviceExtensions(username, tlsca.DeviceExtensions{
			DeviceID:     "deviceid1",
			AssetTag:     "assettag1",
			CredentialID: "credentialid1",
		}))
	require.NoError(t, err, "NewClient failed")

	// updateAuthPref is a helper used throughout the test.
	updateAuthPref := func(t *testing.T, modify func(ap types.AuthPreference)) {
		authPref, err := authServer.GetAuthPreference(ctx)
		require.NoError(t, err, "GetAuthPreference failed")

		modify(authPref)

		_, err = authServer.UpsertAuthPreference(ctx, authPref)
		require.NoError(t, err, "UpsertAuthPreference failed")
	}

	// Enable webauthn
	updateAuthPref(t, func(authPref types.AuthPreference) {
		authPref.SetSecondFactors(
			types.SecondFactorType_SECOND_FACTOR_TYPE_OTP,
			types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
		)
		authPref.SetWebauthn(&types.Webauthn{
			RPID: "localhost",
		})
	})

	assertSuccess := func(t *testing.T, err error) {
		assert.NoError(t, err)
	}
	assertAccessDenied := func(t *testing.T, err error) {
		assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
		assert.ErrorContains(t, err, dtauthz.ErrTrustedDeviceRequired.Error())
	}

	tests := []struct {
		name               string
		clusterDeviceMode  string
		client             *authclient.Client
		skipLoginCerts     bool // aka non-MFA issuance.
		skipSingleUseCerts bool // aka MFA/streaming issuance.
		assertErr          func(t *testing.T, err error)
	}{
		{
			name:              "mode=optional without extensions",
			clusterDeviceMode: constants.DeviceTrustModeOptional,
			client:            clientWithoutDevice,
			assertErr:         assertSuccess,
		},
		{
			name:              "mode=optional with extensions",
			clusterDeviceMode: constants.DeviceTrustModeOptional,
			client:            clientWithDevice,
			assertErr:         assertSuccess,
		},
		{
			name:              "nok: mode=required without extensions",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithoutDevice,
			assertErr:         assertAccessDenied,
		},
		{
			name:              "mode=required with extensions",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithDevice,
			assertErr:         assertSuccess,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updateAuthPref(t, func(ap types.AuthPreference) {
				ap.SetDeviceTrust(&types.DeviceTrust{
					Mode: test.clusterDeviceMode,
				})
			})

			t.Run("CreatePrivilegeTokenRequest", func(t *testing.T) {
				_, err := test.client.CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{})
				test.assertErr(t, err)
			})

			t.Run("CreateRegisterChallenge", func(t *testing.T) {
				_, err := test.client.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
					DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
					DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_MFA,
				})
				test.assertErr(t, err)
			})
		})
	}
}

func mustCreateDatabase(t *testing.T, name, protocol, uri string) *types.DatabaseV3 {
	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: name,
		},
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
		},
	)
	require.NoError(t, err)
	return database
}

func TestGenerateUserCerts_singleUseCerts(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise, // required for IP pinning.
		TestFeatures:  modules.GetModules().Features(),
	})

	ctx := context.Background()
	srv := newTestTLSServer(t)
	clock := srv.Clock()
	userCertTTL := 12 * time.Hour
	userCertExpires := clock.Now().Add(userCertTTL)

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	const webOrigin = "https://localhost" // matches RPID above
	require.NoError(t, err)
	_, err = srv.Auth().UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "node-a",
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	kube, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube-a",
	}, types.KubernetesClusterSpecV3{})

	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "kube-a", "kube-a")
	require.NoError(t, err)
	_, err = srv.Auth().UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)
	// Register a database.

	db, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "db-a",
	}, types.DatabaseServerSpecV3{
		Database: mustCreateDatabase(t, "db-a", "postgres", "localhost"),
		Hostname: "localhost",
		HostID:   "localhost",
	})
	require.NoError(t, err)

	_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
	require.NoError(t, err)

	desktop, err := types.NewWindowsDesktopV3("desktop", nil, types.WindowsDesktopSpecV3{
		Addr:   "localhost",
		HostID: "test",
	})
	require.NoError(t, err)

	require.NoError(t, srv.Auth().CreateWindowsDesktop(ctx, desktop))

	app, err := types.NewAppV3(
		types.Metadata{
			Name: "app-a",
		},
		types.AppSpecV3{
			URI: "localhost",
		},
	)
	require.NoError(t, err)
	appServer, err := types.NewAppServerV3FromApp(app, "app-a", "app-a")
	require.NoError(t, err)
	_, err = srv.Auth().UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	leaf, err := types.NewRemoteCluster("leaf")
	require.NoError(t, err)

	// create remote cluster
	_, err = srv.Auth().CreateRemoteCluster(ctx, leaf)
	require.NoError(t, err)

	// Create a fake user.
	user, role, err := CreateUserAndRole(srv.Auth(), "mfa-user", []string{"role"}, nil)
	require.NoError(t, err)
	// Make sure MFA is required for this user.
	roleOpt := role.GetOptions()
	roleOpt.RequireMFAType = types.RequireMFAType_SESSION
	role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	role.SetWindowsLogins(types.Allow, []string{"role"})
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	role.SetOptions(roleOpt)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	testUser := TestUser(user.GetName())
	testUser.TTL = userCertTTL
	cl, err := srv.NewClient(testUser)
	require.NoError(t, err)

	// Register MFA devices for the fake user.
	registered := addOneOfEachMFADevice(t, cl, clock, webOrigin)
	// Adding MFA devices advances fake clock by 1 minute, here we return it back.
	fakeClock, ok := clock.(*clockwork.FakeClock)
	require.True(t, ok)
	fakeClock.Advance(-60 * time.Second)

	// Fetch MFA device IDs.
	devs, err := srv.Auth().Services.GetMFADevices(ctx, user.GetName(), false)
	require.NoError(t, err)
	var webDevID string
	for _, dev := range devs {
		if dev.GetWebauthn() != nil {
			webDevID = dev.Id
			break
		}
	}

	_, sshPub, _, tlsPub := newSSHAndTLSKeyPairs(t)

	// Used for device trust tests.
	wantDeviceExtensions := tlsca.DeviceExtensions{
		DeviceID:     "device-id1",
		AssetTag:     "device-assettag1",
		CredentialID: "device-credentialid1",
	}

	tests := []struct {
		desc      string
		newClient func() (*authclient.Client, error) // optional, makes a new client for the test.
		opts      generateUserSingleUseCertsTestOpts
	}{
		{
			desc: "ssh using webauthn",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:  clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:    proto.UserCertsRequest_SSH,
					NodeName: "node-a",
					SSHLogin: "role",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					sshCertBytes := c.SSH
					require.NotEmpty(t, sshCertBytes)

					cert, err := sshutils.ParseCertificate(sshCertBytes)
					require.NoError(t, err)

					require.Equal(t, webDevID, cert.Extensions[teleport.CertExtensionMFAVerified])
					require.Equal(t, userCertExpires.Format(time.RFC3339), cert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionLoginIP]).IsLoopback())
					require.Equal(t, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()), cert.ValidBefore)
				},
			},
		},
		{
			desc: "ssh - adjusted expiry",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:  clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:    proto.UserCertsRequest_SSH,
					NodeName: "node-a",
					SSHLogin: "role",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.SSH
					require.NotEmpty(t, crt)

					cert, err := sshutils.ParseCertificate(crt)
					require.NoError(t, err)

					require.Equal(t, webDevID, cert.Extensions[teleport.CertExtensionMFAVerified])
					require.Equal(t, userCertExpires.Format(time.RFC3339), cert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionLoginIP]).IsLoopback())
					require.Equal(t, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()), cert.ValidBefore)
				},
			},
		},
		{
			desc: "fail - single use ssh using totp",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:  clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:    proto.UserCertsRequest_SSH,
					NodeName: "node-a",
					SSHLogin: "role",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.totpAuthHandler,
				verifyErr: func(t require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(t, err, "per-session MFA is not satisfied by 'otp' devices")
				},
			},
		},
		{
			desc: "k8s",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:           clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-a",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageKubeOnly}, identity.Usage)
					require.Equal(t, "kube-a", identity.KubernetesCluster)
				},
			},
		},
		{
			desc: "db",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
						Database:    "db-a",
					},
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, clock.Now().Add(teleport.UserSingleUseCertTTL), cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
					require.Equal(t, "db-a", identity.RouteToDatabase.ServiceName)
				},
			},
		},
		{
			desc: "app",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_App,
					RouteToApp: proto.RouteToApp{
						Name: "app-a",
					},
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageAppsOnly}, identity.Usage)
					require.Equal(t, "app-a", identity.RouteToApp.Name)
					// session ID should be set to a random ID, corresponding to an app session.
					require.NotZero(t, identity.RouteToApp.SessionID)
				},
			},
		},
		{
			desc: "TCP app with target port",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_App,
					RouteToApp: proto.RouteToApp{
						Name:       "app-a",
						TargetPort: 1337,
					},
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageAppsOnly}, identity.Usage)
					require.Equal(t, "app-a", identity.RouteToApp.Name)
					require.Equal(t, 1337, identity.RouteToApp.TargetPort)
					// session ID should be set to a random ID, corresponding to an app session.
					require.NotZero(t, identity.RouteToApp.SessionID)
				},
			},
		},
		{
			desc: "db with ttl limit disabled",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry should *not* be adjusted to single user cert TTL,
					// since ttl limiting is disabled when requester is a local proxy tunnel.
					// It *should* be adjusted to the user cert ttl though.
					Expires: clock.Now().Add(1000 * time.Hour),
					Usage:   proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
					},
					RequesterName: proto.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL,
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, userCertExpires, cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
					require.Equal(t, "db-a", identity.RouteToDatabase.ServiceName)
				},
			},
		},
		{
			desc: "kube with ttl limit disabled",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry should *not* be adjusted to single user cert TTL,
					// since ttl limiting is disabled when requester is a local proxy.
					// It *should* be adjusted to the user cert ttl though.
					Expires:           clock.Now().Add(1000 * time.Hour),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-a",
					RequesterName:     proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY,
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, userCertExpires, cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageKubeOnly}, identity.Usage)
					require.Equal(t, "kube-a", identity.KubernetesCluster)
				},
			},
		},
		{
			desc: "app with ttl limit disabled",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry should *not* be adjusted to single user cert TTL,
					// since ttl limiting is disabled when requester is a local proxy.
					// It *should* be adjusted to the user cert ttl though.
					Expires: clock.Now().Add(1000 * time.Hour),
					Usage:   proto.UserCertsRequest_App,
					RouteToApp: proto.RouteToApp{
						Name: "app-a",
					},
					RequesterName: proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, userCertExpires, cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageAppsOnly}, identity.Usage)
					require.Equal(t, "app-a", identity.RouteToApp.Name)
					// session ID should be set to a random ID, corresponding to an app session.
					require.NotZero(t, identity.RouteToApp.SessionID)
				},
			},
		},
		{
			desc: "desktops",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_WindowsDesktop,
					RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
						WindowsDesktop: "desktop",
						Login:          "role",
					},
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageWindowsDesktopOnly}, identity.Usage)
				},
			},
		},
		{
			desc: "fail - wrong usage",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					Expires:      clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:        proto.UserCertsRequest_All,
					NodeName:     "node-a",
				},
				verifyErr: func(t require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(t, err, "all purposes")
				},
			},
		},
		{
			desc: "fail - mfa challenge fail",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					Expires:      clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:        proto.UserCertsRequest_SSH,
					NodeName:     "node-a",
					SSHLogin:     "role",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Return no challenge response.
					return &proto.MFAAuthenticateResponse{}
				},
				verifyErr: func(t require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(t, err, "unknown or missing MFAAuthenticateResponse")
				},
			},
		},
		{
			desc: "device extensions copied SSH cert",
			newClient: func() (*authclient.Client, error) {
				u := TestUser(user.GetName())
				u.TTL = 1 * time.Hour

				// Add device extensions to the fake user's identity.
				localUser := u.I.(authz.LocalUser)
				localUser.Identity.DeviceExtensions = wantDeviceExtensions
				u.I = localUser

				return srv.NewClient(u)
			},
			opts: generateUserSingleUseCertsTestOpts{
				// Same as SSH options. Nothing special here.
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					Username:     user.GetName(),
					Expires:      clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:        proto.UserCertsRequest_SSH,
					NodeName:     "node-a",
					SSHLogin:     "role",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					// SSH certificate.
					sshRaw := c.SSH
					require.NotEmpty(t, sshRaw, "Got empty single-use SSH certificate")

					sshCert, err := sshutils.ParseCertificate(sshRaw)
					require.NoError(t, err, "ParseCertificate failed")

					gotSSH := tlsca.DeviceExtensions{
						DeviceID:     sshCert.Extensions[teleport.CertExtensionDeviceID],
						AssetTag:     sshCert.Extensions[teleport.CertExtensionDeviceAssetTag],
						CredentialID: sshCert.Extensions[teleport.CertExtensionDeviceCredentialID],
					}
					if diff := cmp.Diff(wantDeviceExtensions, gotSSH, protocmp.Transform()); diff != "" {
						t.Errorf("SSH DeviceExtensions mismatch (-want +got)\n%s", diff)
					}
				},
			},
		},
		{
			desc: "device extensions copied TLS cert",
			newClient: func() (*authclient.Client, error) {
				u := TestUser(user.GetName())
				u.TTL = 1 * time.Hour

				// Add device extensions to the fake user's identity.
				localUser := u.I.(authz.LocalUser)
				localUser.Identity.DeviceExtensions = wantDeviceExtensions
				u.I = localUser

				return srv.NewClient(u)
			},
			opts: generateUserSingleUseCertsTestOpts{
				// Same as Database options. Nothing special here.
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					Expires:      clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:        proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
					},
				},
				authnHandler: registered.webAuthHandler,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				verifyErr: require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					// TLS certificate.
					tlsRaw := c.TLS
					require.NotEmpty(t, tlsRaw, "Got empty single-use TLS certificate")

					block, _ := pem.Decode(tlsRaw)
					require.NotNil(t, block, "Decode failed (TLS PEM)")
					tlsCert, err := x509.ParseCertificate(block.Bytes)
					require.NoError(t, err, "ParseCertificate failed")

					singleUseIdentity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
					require.NoError(t, err, "FromSubject failed")
					gotTLS := singleUseIdentity.DeviceExtensions
					if diff := cmp.Diff(wantDeviceExtensions, gotTLS, protocmp.Transform()); diff != "" {
						t.Errorf("TLS DeviceExtensions mismatch (-want +got)\n%s", diff)
					}
				},
			},
		},
		{
			desc: "mfa unspecified when no SSHLogin provided",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					Expires:      clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:        proto.UserCertsRequest_SSH,
					NodeName:     "node-a",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authnHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Return no challenge response.
					return &proto.MFAAuthenticateResponse{}
				},
				verifyErr: func(t require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(t, err, "unknown or missing MFAAuthenticateResponse")
				},
			},
		},
		{
			desc: "k8s in leaf cluster",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:           clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-b",
					RouteToCluster:    "leaf",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageKubeOnly}, identity.Usage)
					require.Equal(t, "kube-b", identity.KubernetesCluster)
				},
			},
		},
		{
			desc: "db in leaf cluster",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-b",
						Database:    "db-b",
					},
					RouteToCluster: "leaf",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, clock.Now().Add(teleport.UserSingleUseCertTTL), cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
					require.Equal(t, "db-b", identity.RouteToDatabase.ServiceName)
				},
			},
		},
		{
			desc: "app in leaf cluster",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					TLSPublicKey: tlsPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_App,
					RouteToApp: proto.RouteToApp{
						Name: "app-b",
					},
					RouteToCluster: "leaf",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					crt := c.TLS
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageAppsOnly}, identity.Usage)
					require.Equal(t, "app-b", identity.RouteToApp.Name)
					// session ID should be set to a random ID, corresponding to an app session.
					require.NotZero(t, identity.RouteToApp.SessionID)
				},
			},
		},
		{
			desc: "ssh in leaf node",
			opts: generateUserSingleUseCertsTestOpts{
				initReq: &proto.UserCertsRequest{
					SSHPublicKey: sshPub,
					Username:     user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:        clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:          proto.UserCertsRequest_SSH,
					NodeName:       "node-b",
					SSHLogin:       "role",
					RouteToCluster: "leaf",
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authnHandler: registered.webAuthHandler,
				verifyErr:    require.NoError,
				verifyCert: func(t *testing.T, c *proto.Certs) {
					sshCertBytes := c.SSH
					require.NotEmpty(t, sshCertBytes)

					cert, err := sshutils.ParseCertificate(sshCertBytes)
					require.NoError(t, err)

					require.Equal(t, webDevID, cert.Extensions[teleport.CertExtensionMFAVerified])
					require.Equal(t, userCertExpires.Format(time.RFC3339), cert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionLoginIP]).IsLoopback())
					require.Equal(t, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()), cert.ValidBefore)
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			testClient := cl
			if tt.newClient != nil {
				var err error
				testClient, err = tt.newClient()
				require.NoError(t, err, "newClient failed")
			}

			testGenerateUserSingleUseCerts(ctx, t, testClient, tt.opts)
		})
	}
}

type generateUserSingleUseCertsTestOpts struct {
	initReq            *proto.UserCertsRequest
	authnHandler       func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	mfaRequiredHandler func(*testing.T, proto.MFARequired)
	verifyErr          require.ErrorAssertionFunc
	verifyCert         func(*testing.T, *proto.Certs)
}

func testGenerateUserSingleUseCerts(ctx context.Context, t *testing.T, cl *authclient.Client, opts generateUserSingleUseCertsTestOpts) {
	authnChal, err := cl.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
	})
	require.NoError(t, err, "CreateAuthenticateChallenge")

	req := opts.initReq
	req.Purpose = proto.UserCertsRequest_CERT_PURPOSE_SINGLE_USE_CERTS
	if opts.authnHandler != nil {
		req.MFAResponse = opts.authnHandler(t, authnChal)
	}

	certs, err := cl.GenerateUserCerts(ctx, *req)
	opts.verifyErr(t, err)
	if err != nil {
		return
	}

	opts.verifyCert(t, certs)
}

var requireMFATypes = []types.RequireMFAType{
	types.RequireMFAType_OFF,
	types.RequireMFAType_SESSION,
	types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
	types.RequireMFAType_HARDWARE_KEY_TOUCH,
	types.RequireMFAType_HARDWARE_KEY_PIN,
	types.RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN,
}

func TestIsMFARequired(t *testing.T) {
	testModules := &modules.TestModules{
		TestBuildType:       modules.BuildEnterprise,
		MockAttestationData: &keys.AttestationData{},
	}
	modules.SetTestModules(t, testModules)

	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: uuid.NewString(),
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err := srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	for _, authPrefRequireMFAType := range requireMFATypes {
		t.Run(fmt.Sprintf("authPref=%v", authPrefRequireMFAType.String()), func(t *testing.T) {
			authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				Type:           constants.Local,
				SecondFactor:   constants.SecondFactorOptional,
				RequireMFAType: authPrefRequireMFAType,
				Webauthn: &types.Webauthn{
					RPID: "teleport",
				},
			})
			require.NoError(t, err)
			authPref, err = srv.Auth().UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			for _, roleRequireMFAType := range requireMFATypes {
				roleRequireMFAType := roleRequireMFAType
				t.Run(fmt.Sprintf("role=%v", roleRequireMFAType.String()), func(t *testing.T) {
					user, err := types.NewUser(roleRequireMFAType.String())
					require.NoError(t, err)

					role := services.RoleForUser(user)
					roleOpt := role.GetOptions()
					roleOpt.RequireMFAType = roleRequireMFAType
					role.SetOptions(roleOpt)
					role.SetLogins(types.Allow, []string{user.GetName()})

					role, err = srv.Auth().UpsertRole(ctx, role)
					require.NoError(t, err)

					user.AddRole(role.GetName())
					user, err = srv.Auth().UpsertUser(ctx, user)
					require.NoError(t, err)

					mfaVerifiedByHardwareKey := role.GetPrivateKeyPolicy().MFAVerified() || authPref.GetPrivateKeyPolicy().MFAVerified()
					if mfaVerifiedByHardwareKey {
						// Set attestated key policy to the most restrictive hardware key MFA is required.
						testModules.MockAttestationData.PrivateKeyPolicy = keys.PrivateKeyPolicyHardwareKeyTouchAndPIN
					} else {
						testModules.MockAttestationData.PrivateKeyPolicy = keys.PrivateKeyPolicyHardwareKey
					}

					cl, err := srv.NewClient(TestUser(user.GetName()))
					require.NoError(t, err)

					resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
						Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
							Login: user.GetName(),
							Node:  "node-a",
						}},
					})
					require.NoError(t, err)

					// If auth pref or role require session MFA, and MFA is not already
					// verified according to private key policy, expect MFA required.
					wantRequired := (role.GetOptions().RequireMFAType.IsSessionMFARequired() || authPref.GetRequireMFAType().IsSessionMFARequired()) && !mfaVerifiedByHardwareKey
					var wantMFARequired proto.MFARequired
					if wantRequired {
						wantMFARequired = proto.MFARequired_MFA_REQUIRED_YES
					} else {
						wantMFARequired = proto.MFARequired_MFA_REQUIRED_NO
					}
					assert.Equal(t, wantRequired, resp.Required, "Required mismatch")
					assert.Equal(t, wantMFARequired, resp.MFARequired, "IsMFARequired mismatch")
				})
			}
		})
	}
}

func TestIsMFARequired_unauthorized(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "teleport",
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node1 := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node1",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"a": "b"},
		},
		Spec: types.ServerSpecV2{
			Hostname: "node1",
			Addr:     "localhost:3022",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node1)
	require.NoError(t, err)

	// Register another SSH node with a duplicate hostname.
	node2 := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node2",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"a": "c"},
		},
		Spec: types.ServerSpecV2{
			Hostname: "node1",
			Addr:     "localhost:3022",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node2)
	require.NoError(t, err)

	user, role, err := CreateUserAndRole(srv.Auth(), "alice", []string{"alice"}, nil)
	require.NoError(t, err)

	// Require MFA.
	roleOpt := role.GetOptions()
	roleOpt.RequireMFAType = types.RequireMFAType_SESSION
	role.SetOptions(roleOpt)
	role.SetNodeLabels(types.Allow, map[string]utils.Strings{"a": []string{"c"}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Call the endpoint for an authorized login. The user is only authorized
	// for the 2nd node, but should still be asked for MFA.
	resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
			Login: "alice",
			Node:  "node1",
		}},
	})
	require.NoError(t, err, "IsMFARequired")
	assert.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, resp.MFARequired, "MFARequired mismatch")
	assert.True(t, resp.Required, "Required mismatch")

	// Call the endpoint for an unauthorized login.
	resp, err = cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
			Login: "bob",
			Node:  "node1",
		}},
	})
	require.NoError(t, err, "IsMFARequired silent failure wanted")
	assert.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, resp.MFARequired, "MFARequired mismatch")
	assert.False(t, resp.Required, "Required mismatch")
}

func TestIsMFARequired_nodeMatch(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Register an SSH node.
	node, err := types.NewServerWithLabels(uuid.NewString(), types.KindNode, types.ServerSpecV2{
		Hostname:    "node-a",
		Addr:        "127.0.0.1:3022",
		PublicAddrs: []string{"node.example.com:3022", "localhost:3022"},
	}, map[string]string{"foo": "bar"})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	// Create a fake user with per session mfa required for all nodes.
	role, err := CreateRole(ctx, srv.Auth(), "mfa-user", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			Logins:     []string{"mfa-user"},
			NodeLabels: types.Labels{types.Wildcard: utils.Strings{types.Wildcard}},
		},
	})
	require.NoError(t, err)

	user, err := CreateUser(ctx, srv.Auth(), "mfa-user", role)
	require.NoError(t, err)

	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, tc := range []struct {
		desc string
		// IsMFARequired only expects a host name or ip without the port.
		node string
		want proto.MFARequired
	}{
		{
			desc: "OK uuid match",
			node: node.GetName(),
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
		{
			desc: "OK host name match",
			node: node.GetHostname(),
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
		{
			desc: "OK addr match",
			node: node.GetAddr(),
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
		{
			desc: "OK public addr 1 match",
			node: "node.example.com",
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
		{
			desc: "OK public addr 2 match",
			node: "localhost",
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
		{
			desc: "NOK label match",
			node: "foo",
			want: proto.MFARequired_MFA_REQUIRED_NO,
		},
		{
			desc: "NOK unknown ip",
			node: "1.2.3.4",
			want: proto.MFARequired_MFA_REQUIRED_NO,
		},
		{
			desc: "NOK unknown addr",
			node: "unknown.example.com",
			want: proto.MFARequired_MFA_REQUIRED_NO,
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
					Login: user.GetName(),
					Node:  tc.node,
				}},
			})
			require.NoError(t, err, "IsMFARequired")

			assert.Equal(t, tc.want, resp.MFARequired, "MFARequired mismatch")
			assert.Equal(t, MFARequiredToBool(tc.want), resp.Required, "Required mismatch")
		})
	}
}

func TestIsMFARequired_App(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	appA, err := types.NewAppV3(types.Metadata{
		Name: "app-a",
		Labels: map[string]string{
			"env": "dev",
		},
	}, types.AppSpecV3{
		URI:        "app-a",
		PublicAddr: "appa.example.com",
	})
	require.NoError(t, err)
	appAServer, err := types.NewAppServerV3FromApp(appA, "host", "host-id")
	require.NoError(t, err)
	_, err = srv.Auth().UpsertApplicationServer(ctx, appAServer)
	require.NoError(t, err)

	accessDevApp, err := types.NewRole("access-a", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"env": []string{"dev"},
			},
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, accessDevApp)
	require.NoError(t, err)

	accessDevAppMFA, err := types.NewRole("access-a-mfa", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"env": []string{"dev"},
			},
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, accessDevAppMFA)
	require.NoError(t, err)

	accessStagingAppMFA, err := types.NewRole("access-b-mfa", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"env": []string{"staging"},
			},
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, accessStagingAppMFA)
	require.NoError(t, err)

	for _, tc := range []struct {
		name              string
		authPrefMFAType   types.RequireMFAType
		roles             []string
		expectMFARequired proto.MFARequired
	}{
		{
			name:              "mfa not required",
			authPrefMFAType:   types.RequireMFAType_OFF,
			roles:             []string{accessDevApp.GetName()},
			expectMFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}, {
			name:              "mfa required by role",
			authPrefMFAType:   types.RequireMFAType_OFF,
			roles:             []string{accessDevAppMFA.GetName()},
			expectMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
		}, {
			name:              "mfa required by cap",
			authPrefMFAType:   types.RequireMFAType_SESSION,
			roles:             []string{accessDevApp.GetName()},
			expectMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
		}, {
			name:              "mfa not required from unmatched role role",
			authPrefMFAType:   types.RequireMFAType_OFF,
			roles:             []string{accessDevApp.GetName(), accessStagingAppMFA.GetName()},
			expectMFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		},
	} {
		// Update auth pref mfa type
		authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
			Type:           constants.Local,
			SecondFactor:   constants.SecondFactorOptional,
			RequireMFAType: tc.authPrefMFAType,
			Webauthn: &types.Webauthn{
				RPID: "teleport",
			},
		})
		require.NoError(t, err)
		_, err = srv.Auth().UpsertAuthPreference(ctx, authPref)
		require.NoError(t, err)

		// Create a user with test case roles.
		user, err := types.NewUser("alice")
		require.NoError(t, err)
		user.SetRoles(tc.roles)
		user, err = srv.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)

		cl, err := srv.NewClient(TestUser(user.GetName()))
		require.NoError(t, err)

		resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_App{App: &proto.RouteToApp{
				Name: "app-a",
			}},
		})
		require.NoError(t, err)

		assert.Equal(t, tc.expectMFARequired, resp.MFARequired, "IsMFARequired mismatch")
		assert.Equal(t, tc.expectMFARequired == proto.MFARequired_MFA_REQUIRED_YES, resp.Required, "Required mismatch")
	}
}

// testOriginDynamicStored tests setting a ResourceWithOrigin via the server
// API always results in the resource being stored with OriginDynamic.
func testOriginDynamicStored(t *testing.T, setWithOrigin func(*authclient.Client, string) error, getStored func(*Server) (types.ResourceWithOrigin, error)) {
	srv := newTestTLSServer(t)

	// Create a fake user.
	user, _, err := CreateUserAndRole(srv.Auth(), "configurer", []string{}, nil)
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, origin := range types.OriginValues {
		t.Run(fmt.Sprintf("setting with origin %q", origin), func(t *testing.T) {
			err := setWithOrigin(cl, origin)
			require.NoError(t, err)

			stored, err := getStored(srv.Auth())
			require.NoError(t, err)
			require.Equal(t, types.OriginDynamic, stored.Origin())
		})
	}
}

func TestAuthPreferenceOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *authclient.Client, origin string) error {
		authPref := types.DefaultAuthPreference()
		authPref.SetOrigin(origin)
		_, err := cl.UpsertAuthPreference(ctx, authPref)
		return err
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetAuthPreference(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestClusterNetworkingConfigOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *authclient.Client, origin string) error {
		netConfig := types.DefaultClusterNetworkingConfig()
		netConfig.SetOrigin(origin)
		_, err := cl.UpsertClusterNetworkingConfig(ctx, netConfig)
		return trace.Wrap(err)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetClusterNetworkingConfig(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestSessionRecordingConfigOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *authclient.Client, origin string) error {
		recConfig := types.DefaultSessionRecordingConfig()
		recConfig.SetOrigin(origin)
		_, err := cl.UpsertSessionRecordingConfig(ctx, recConfig)
		return err
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetSessionRecordingConfig(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestGenerateHostCerts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	pubTLS, err := PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	certs, err := clt.GenerateHostCerts(ctx, &proto.HostCertsRequest{
		HostID:   "Admin",
		Role:     types.RoleAdmin,
		NodeName: "foo",
		// Ensure that 0.0.0.0 gets replaced with the RemoteAddr of the client
		AdditionalPrincipals: []string{"0.0.0.0"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
	})
	require.NoError(t, err)
	require.NotNil(t, certs)
}

func TestGenerateDatabaseCerts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Generate CSR once for speed sake.
	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)
	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{CommonName: "test"}, priv)
	require.NoError(t, err)

	certs, err := clt.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{CSR: csr})
	require.NoError(t, err)
	require.NotNil(t, certs)

	certs, err = clt.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{CSR: csr, RequesterName: proto.DatabaseCertRequest_TCTL})
	require.NoError(t, err)
	require.NotNil(t, certs)
}

// TestInstanceCertAndControlStream attempts to generate an instance cert via the
// assertion API and use it to handle an inventory ping via the control stream.
func TestInstanceCertAndControlStream(t *testing.T) {
	const assertionID = "test-assertion"
	const serverID = "test-server"
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newTestTLSServer(t)

	roles := []types.SystemRole{
		types.RoleNode,
		types.RoleAuth,
		types.RoleProxy,
	}

	clt, err := srv.NewClient(TestServerID(types.RoleNode, serverID))
	require.NoError(t, err)
	defer clt.Close()

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	pubTLS, err := PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	req := proto.HostCertsRequest{
		HostID:       serverID,
		Role:         types.RoleInstance,
		PublicSSHKey: pub,
		PublicTLSKey: pubTLS,
		SystemRoles:  roles,
		// assertion ID is omitted initially to test
		// the failure case
	}

	// request should fail since clt only holds RoleNode
	_, err = clt.GenerateHostCerts(ctx, &req)
	require.True(t, trace.IsAccessDenied(err))

	// perform assertions
	for _, role := range roles {
		func() {
			clt, err := srv.NewClient(TestServerID(role, serverID))
			require.NoError(t, err)
			defer clt.Close()

			err = clt.AssertSystemRole(ctx, proto.SystemRoleAssertion{
				ServerID:    serverID,
				AssertionID: assertionID,
				SystemRole:  role,
			})
			require.NoError(t, err)
		}()
	}

	// set assertion ID
	req.SystemRoleAssertionID = assertionID

	// assertion should allow us to generate certs
	certs, err := clt.GenerateHostCerts(ctx, &req)
	require.NoError(t, err)

	// make an instance client
	instanceCert, err := tls.X509KeyPair(certs.TLS, priv)
	require.NoError(t, err)
	instanceClt := srv.NewClientWithCert(instanceCert)

	// instance cert can self-renew without assertions
	req.SystemRoleAssertionID = ""
	_, err = instanceClt.GenerateHostCerts(ctx, &req)
	require.NoError(t, err)

	stream, err := instanceClt.InventoryControlStream(ctx)
	require.NoError(t, err)
	defer stream.Close()

	err = stream.Send(ctx, proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: roles,
	})
	require.NoError(t, err)

	select {
	case msg := <-stream.Recv():
		_, ok := msg.(proto.DownstreamInventoryHello)
		require.True(t, ok)
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for downstream hello")
	}

	// fire off a ping in the background
	pingErr := make(chan error, 1)
	go func() {
		defer close(pingErr)
		// get an admin client so that we can test pings
		clt, err := srv.NewClient(TestAdmin())
		if err != nil {
			pingErr <- err
			return
		}
		defer clt.Close()

		_, err = clt.PingInventory(ctx, proto.InventoryPingRequest{
			ServerID: serverID,
		})
		pingErr <- err
	}()

	// wait for the ping
	select {
	case msg := <-stream.Recv():
		ping, ok := msg.(proto.DownstreamInventoryPing)
		require.True(t, ok)
		err = stream.Send(ctx, proto.UpstreamInventoryPong{
			ID: ping.ID,
		})
		require.NoError(t, err)
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for downstream ping")
	}

	// ensure that bg ping routine was successful
	require.NoError(t, <-pingErr)
}

func TestGetSSHTargets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	upper, err := types.NewServerWithLabels(uuid.New().String(), types.KindNode, types.ServerSpecV2{
		Hostname:  "Foo",
		UseTunnel: true,
	}, nil)
	require.NoError(t, err)

	lower, err := types.NewServerWithLabels(uuid.New().String(), types.KindNode, types.ServerSpecV2{
		Hostname:  "foo",
		UseTunnel: true,
	}, nil)
	require.NoError(t, err)

	other, err := types.NewServerWithLabels(uuid.New().String(), types.KindNode, types.ServerSpecV2{
		Hostname:  "bar",
		UseTunnel: true,
	}, nil)
	require.NoError(t, err)

	for _, node := range []types.Server{upper, lower, other} {
		_, err = clt.UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	rsp, err := clt.GetSSHTargets(ctx, &proto.GetSSHTargetsRequest{
		Host: "foo",
		Port: "0",
	})
	require.NoError(t, err)
	require.Len(t, rsp.Servers, 1)
	require.Equal(t, "foo", rsp.Servers[0].GetHostname())

	cnc := types.DefaultClusterNetworkingConfig()
	cnc.SetCaseInsensitiveRouting(true)
	_, err = clt.UpsertClusterNetworkingConfig(ctx, cnc)
	require.NoError(t, err)

	rsp, err = clt.GetSSHTargets(ctx, &proto.GetSSHTargetsRequest{
		Host: "foo",
		Port: "0",
	})
	require.NoError(t, err)
	require.Len(t, rsp.Servers, 2)
	require.ElementsMatch(t, []string{rsp.Servers[0].GetHostname(), rsp.Servers[1].GetHostname()}, []string{"foo", "Foo"})
}

func TestResolveSSHTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	upper, err := types.NewServerWithLabels(uuid.New().String(), types.KindNode, types.ServerSpecV2{
		Hostname:  "Foo",
		UseTunnel: true,
	}, nil)
	require.NoError(t, err)
	upper.SetExpiry(time.Now().Add(time.Hour))

	lower, err := types.NewServerWithLabels(uuid.New().String(), types.KindNode, types.ServerSpecV2{
		Hostname:  "foo",
		UseTunnel: true,
	}, nil)
	require.NoError(t, err)

	other, err := types.NewServerWithLabels(uuid.New().String(), types.KindNode, types.ServerSpecV2{
		Hostname:  "bar",
		UseTunnel: true,
	}, nil)
	require.NoError(t, err)

	for _, node := range []types.Server{upper, lower, other} {
		_, err = clt.UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	rsp, err := clt.ResolveSSHTarget(ctx, &proto.ResolveSSHTargetRequest{
		Host: "foo",
		Port: "0",
	})
	require.NoError(t, err)
	require.Equal(t, "foo", rsp.Server.GetHostname())

	cnc := types.DefaultClusterNetworkingConfig()
	cnc.SetCaseInsensitiveRouting(true)
	_, err = clt.UpsertClusterNetworkingConfig(ctx, cnc)
	require.NoError(t, err)

	rsp, err = clt.ResolveSSHTarget(ctx, &proto.ResolveSSHTargetRequest{
		Host: "foo",
		Port: "0",
	})
	require.Error(t, err)
	require.Nil(t, rsp)

	cnc.SetRoutingStrategy(types.RoutingStrategy_MOST_RECENT)
	_, err = clt.UpsertClusterNetworkingConfig(ctx, cnc)
	require.NoError(t, err)

	rsp, err = clt.ResolveSSHTarget(ctx, &proto.ResolveSSHTargetRequest{
		Host: "foo",
		Port: "0",
	})
	require.NoError(t, err)
	require.Equal(t, "Foo", rsp.Server.GetHostname())
}

func TestNodesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// node1 and node2 will be added to default namespace
	node1, err := types.NewServerWithLabels("node1", types.KindNode, types.ServerSpecV2{Hostname: "node1"}, nil)
	require.NoError(t, err)
	node2, err := types.NewServerWithLabels("node2", types.KindNode, types.ServerSpecV2{Hostname: "node2"}, nil)
	require.NoError(t, err)

	t.Run("CreateNode", func(t *testing.T) {
		// Initially expect no nodes to be returned.
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Empty(t, nodes)

		// Create nodes.
		_, err = clt.UpsertNode(ctx, node1)
		require.NoError(t, err)

		_, err = clt.UpsertNode(ctx, node2)
		require.NoError(t, err)
	})

	// Run NodeGetters in nested subtests to allow parallelization.
	t.Run("NodeGetters", func(t *testing.T) {
		t.Run("GetNodes", func(t *testing.T) {
			t.Parallel()
			// Get all nodes
			nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			require.Len(t, nodes, 2)
			require.Empty(t, cmp.Diff([]types.Server{node1, node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// GetNodes should not fail if namespace is empty
			_, err = clt.GetNodes(ctx, "")
			require.NoError(t, err)
		})
		t.Run("GetNode", func(t *testing.T) {
			t.Parallel()
			// Get Node
			node, err := clt.GetNode(ctx, apidefaults.Namespace, "node1")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(node1, node,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// GetNode should fail if node name isn't provided
			_, err = clt.GetNode(ctx, apidefaults.Namespace, "")
			require.True(t, trace.IsBadParameter(err), "trace.IsBadParameter failed: err=%v (%T)", err, trace.Unwrap(err))

			// GetNode should fail if namespace isn't provided
			_, err = clt.GetNode(ctx, "", "node1")
			require.True(t, trace.IsBadParameter(err), "trace.IsBadParameter failed: err=%v (%T)", err, trace.Unwrap(err))
		})
	})

	t.Run("DeleteNode", func(t *testing.T) {
		// Make sure can't delete with empty namespace or name.
		err = clt.DeleteNode(ctx, apidefaults.Namespace, "")
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		err = clt.DeleteNode(ctx, "", node1.GetName())
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		// Delete node.
		err = clt.DeleteNode(ctx, apidefaults.Namespace, node1.GetName())
		require.NoError(t, err)

		// Expect node not found
		_, err := clt.GetNode(ctx, apidefaults.Namespace, "node1")
		require.IsType(t, trace.NotFound(""), err)
	})

	t.Run("DeleteAllNodes", func(t *testing.T) {
		// Make sure can't delete with empty namespace.
		err = clt.DeleteAllNodes(ctx, "")
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		// Delete nodes
		err = clt.DeleteAllNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)

		// Now expect no nodes to be returned.
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Empty(t, nodes)
	})
}

func TestLocksCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	now := srv.Clock().Now()
	lock1, err := types.NewLock("lock1", types.LockSpecV2{
		Target: types.LockTarget{
			User: "user-A",
		},
		Expires: &now,
	})
	require.NoError(t, err)
	lock1.SetCreatedBy(string(types.RoleAdmin))
	lock1.SetCreatedAt(now)

	lock2, err := types.NewLock("lock2", types.LockSpecV2{
		Target: types.LockTarget{
			Node: "node",
		},
		Message: "node compromised",
	})
	require.NoError(t, err)
	lock2.SetCreatedBy(string(types.RoleAdmin))
	lock2.SetCreatedAt(now)

	t.Run("CreateLock", func(t *testing.T) {
		// Initially expect no locks to be returned.
		locks, err := clt.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)

		// Create locks.
		err = clt.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		err = clt.UpsertLock(ctx, lock2)
		require.NoError(t, err)
	})

	// Run LockGetters in nested subtests to allow parallelization.
	t.Run("LockGetters", func(t *testing.T) {
		t.Run("GetLocks", func(t *testing.T) {
			t.Parallel()
			locks, err := clt.GetLocks(ctx, false)
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
		})
		t.Run("GetLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, err := clt.GetLocks(ctx, false, lock1.Target(), lock2.Target())
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = clt.GetLocks(ctx, false, lock1.Target(), roleTarget)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match none of the locks.
			locks, err = clt.GetLocks(ctx, false, roleTarget)
			require.NoError(t, err)
			require.Empty(t, locks)
		})
		t.Run("GetLock", func(t *testing.T) {
			t.Parallel()
			// Get one of the locks.
			lock, err := clt.GetLock(ctx, lock1.GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(lock1, lock,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Attempt to get a nonexistent lock.
			_, err = clt.GetLock(ctx, "lock3")
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})
	})

	t.Run("UpsertLock", func(t *testing.T) {
		// Get one of the locks.
		lock, err := clt.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Empty(t, lock.Message())

		msg := "cluster maintenance"
		lock1.SetMessage(msg)
		err = clt.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		lock, err = clt.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Equal(t, msg, lock.Message())
	})

	t.Run("DeleteLock", func(t *testing.T) {
		// Delete lock.
		err = clt.DeleteLock(ctx, lock1.GetName())
		require.NoError(t, err)

		// Expect lock not found.
		_, err := clt.GetLock(ctx, lock1.GetName())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

// TestApplicationServersCRUD tests application server operations.
func TestApplicationServersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple app servers.
	app1, err := types.NewAppV3(types.Metadata{Name: "app-1"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server1, err := types.NewAppServerV3FromApp(app1, "server-1", "server-1")
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{Name: "app-2"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server2, err := types.NewAppServerV3FromApp(app2, "server-2", "server-2")
	require.NoError(t, err)
	app3, err := types.NewAppV3(types.Metadata{Name: "app-3"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server3, err := types.NewAppServerV3FromApp(app3, "server-3", "server-3")
	require.NoError(t, err)

	// Initially we expect no app servers.
	out, err := clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)

	// Register all app servers.
	_, err = clt.UpsertApplicationServer(ctx, server1)
	require.NoError(t, err)
	_, err = clt.UpsertApplicationServer(ctx, server2)
	require.NoError(t, err)
	_, err = clt.UpsertApplicationServer(ctx, server3)
	require.NoError(t, err)

	// Fetch all app servers.
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server1, server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Update an app server.
	server1.Metadata.Description = "description"
	_, err = clt.UpsertApplicationServer(ctx, server1)
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server1, server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete an app server.
	err = clt.DeleteApplicationServer(ctx, server1.GetNamespace(), server1.GetHostID(), server1.GetName())
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete all app servers.
	err = clt.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestAppsCRUD tests application resource operations.
func TestAppsCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple apps.
	app1, err := types.NewAppV3(types.Metadata{
		Name:   "app1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{
		Name:   "app2",
		Labels: map[string]string{types.OriginLabel: types.OriginOkta}, // This should be overwritten
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)

	// Initially we expect no apps.
	out, err := clt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Create both apps.
	err = clt.CreateApp(ctx, app1)
	require.NoError(t, err)
	err = clt.CreateApp(ctx, app2)
	require.NoError(t, err)

	// Fetch all apps.
	out, err = clt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app1, app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific app.
	app, err := clt.GetApp(ctx, app2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(app2, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to fetch an app that doesn't exist.
	_, err = clt.GetApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same app.
	err = clt.CreateApp(ctx, app1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update an app.
	app1.Metadata.Description = "description"
	err = clt.UpdateApp(ctx, app1)
	require.NoError(t, err)
	app, err = clt.GetApp(ctx, app1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(app1, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete an app.
	err = clt.DeleteApp(ctx, app1.GetName())
	require.NoError(t, err)
	out, err = clt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete an app that doesn't exist.
	err = clt.DeleteApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all apps.
	err = clt.DeleteAllApps(ctx)
	require.NoError(t, err)
	out, err = clt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestAppServersCRUD tests application server resource operations.
func TestAppServersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create an app server, expected origin dynamic.
	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	app1, err := types.NewAppV3(types.Metadata{
		Name: "app-dynamic",
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)

	appServer1, err := types.NewAppServerV3FromApp(app1, "app-dynamic", "hostID")
	require.NoError(t, err)

	_, err = clt.UpsertApplicationServer(ctx, appServer1)
	require.NoError(t, err)

	resources, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 1)

	appServer := resources.Resources[0].(types.AppServer)
	require.Empty(t, cmp.Diff(appServer, appServer1,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	require.NoError(t, clt.DeleteApplicationServer(ctx, apidefaults.Namespace, "hostID", appServer1.GetName()))

	resources, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Empty(t, resources.Resources)

	// Try to create app servers with Okta labels as a non-Okta role.
	app2, err := types.NewAppV3(types.Metadata{
		Name:   "app-okta",
		Labels: map[string]string{types.OriginLabel: types.OriginOkta},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)

	appServer2, err := types.NewAppServerV3FromApp(app2, "app-okta", "hostID")
	require.NoError(t, err)

	_, err = clt.UpsertApplicationServer(ctx, appServer2)
	require.ErrorIs(t, err, trace.BadParameter("only the Okta role can create app servers and apps with an Okta origin"))

	delete(app2.Metadata.Labels, types.OriginLabel)
	appServer2.SetOrigin(types.OriginOkta)

	_, err = clt.UpsertApplicationServer(ctx, appServer2)
	require.ErrorIs(t, err, trace.BadParameter("only the Okta role can create app servers and apps with an Okta origin"))

	// Create an app server with Okta labels using the Okta role.
	clt, err = srv.NewClient(TestBuiltin(types.RoleOkta))
	require.NoError(t, err)

	app2.SetOrigin(types.OriginOkta)
	appServer2.SetOrigin(types.OriginOkta)
	_, err = clt.UpsertApplicationServer(ctx, appServer2)
	require.NoError(t, err)

	resources, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 1)

	appServer2.SetOrigin(types.OriginOkta)
	app2.SetOrigin(types.OriginOkta)
	appServer = resources.Resources[0].(types.AppServer)
	require.Empty(t, cmp.Diff(appServer, appServer2,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	require.NoError(t, clt.DeleteApplicationServer(ctx, apidefaults.Namespace, "hostID", appServer2.GetName()))

	resources, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Empty(t, resources.Resources)
}

// TestDatabasesCRUD tests database resource operations.
func TestDatabasesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple databases.
	db1, err := types.NewDatabaseV3(types.Metadata{
		Name:   "db1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	db2, err := types.NewDatabaseV3(types.Metadata{
		Name:   "db2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// Initially we expect no databases.
	out, err := clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Create both databases.
	err = clt.CreateDatabase(ctx, db1)
	require.NoError(t, err)
	err = clt.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// Fetch all databases.
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific database.
	db, err := clt.GetDatabase(ctx, db2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db2, db,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to fetch a database that doesn't exist.
	_, err = clt.GetDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same database.
	err = clt.CreateDatabase(ctx, db1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a database.
	db1.Metadata.Description = "description"
	err = clt.UpdateDatabase(ctx, db1)
	require.NoError(t, err)
	db, err = clt.GetDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db1, db,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete a database.
	err = clt.DeleteDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete a database that doesn't exist.
	err = clt.DeleteDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all databases.
	err = clt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestDatabaseServicesCRUD tests DatabaseService resource operations.
func TestDatabaseServicesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create two DatabaseServices.
	db1, err := types.NewDatabaseServiceV1(types.Metadata{
		Name:   "db1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"prod"},
				},
			},
		},
	})
	require.NoError(t, err)

	db2, err := types.NewDatabaseServiceV1(types.Metadata{
		Name:   "db2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"stg"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Initially we expect no DatabaseServices.
	listServicesResp, err := clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err := types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, out)

	// Create both DatabaseServices.
	_, err = clt.UpsertDatabaseService(ctx, db1)
	require.NoError(t, err)
	_, err = clt.UpsertDatabaseService(ctx, db2)
	require.NoError(t, err)

	// Fetch all DatabaseServices.
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseService{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Update a DatabaseService.
	db1.Spec.ResourceMatchers[0] = &types.DatabaseResourceMatcher{
		Labels: &types.Labels{
			"env": []string{"notprod"},
		},
	}

	_, err = clt.UpsertDatabaseService(ctx, db1)
	require.NoError(t, err)
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseService{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete a DatabaseService.
	err = clt.DeleteDatabaseService(ctx, db1.GetName())
	require.NoError(t, err)
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseService{db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete a DatabaseService that doesn't exist.
	err = clt.DeleteDatabaseService(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all DatabaseServices.
	err = clt.DeleteAllDatabaseServices(ctx)
	require.NoError(t, err)
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestServerInfoCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	serverInfo1, err := types.NewServerInfo(types.Metadata{
		Name: "serverInfo1",
	}, types.ServerInfoSpecV1{})
	require.NoError(t, err)
	serverInfo1.SetSubKind(types.SubKindCloudInfo)
	serverInfo2, err := types.NewServerInfo(types.Metadata{
		Name: "serverInfo2",
	}, types.ServerInfoSpecV1{})
	require.NoError(t, err)
	serverInfo2.SetSubKind(types.SubKindCloudInfo)

	createServerInfos := func(t *testing.T) {
		// Initially expect no server info to be returned.
		serverInfos, err := stream.Collect(clt.GetServerInfos(ctx))
		require.NoError(t, err)
		require.Empty(t, serverInfos)

		// Create server info.
		require.NoError(t, clt.UpsertServerInfo(ctx, serverInfo1))
		require.NoError(t, clt.UpsertServerInfo(ctx, serverInfo2))
	}

	deleteAllServerInfos := func(t *testing.T) {
		// Delete server infos.
		require.NoError(t, clt.DeleteAllServerInfos(ctx))

		// Expect no server infos to be returned.
		serverInfos, err := stream.Collect(clt.GetServerInfos(ctx))
		require.NoError(t, err)
		require.Empty(t, serverInfos)
	}

	requireResourcesEqual := func(t *testing.T, expected, actual interface{}) {
		require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	}

	t.Run("ServerInfoGetters", func(t *testing.T) {
		createServerInfos(t)
		t.Cleanup(func() { deleteAllServerInfos(t) })

		t.Run("GetServerInfos", func(t *testing.T) {
			t.Parallel()
			// Get all server infos.
			serverInfos, err := stream.Collect(clt.GetServerInfos(ctx))
			require.NoError(t, err)
			require.Len(t, serverInfos, 2)
			requireResourcesEqual(t, []types.ServerInfo{serverInfo1, serverInfo2}, serverInfos)
		})

		t.Run("GetServerInfo", func(t *testing.T) {
			t.Parallel()
			// Get server info.
			si, err := clt.GetServerInfo(ctx, serverInfo1.GetName())
			require.NoError(t, err)
			requireResourcesEqual(t, serverInfo1, si)

			// GetServerInfo should fail if name isn't provided.
			_, err = clt.GetServerInfo(ctx, "")
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err))
		})
	})

	t.Run("DeleteServerInfo", func(t *testing.T) {
		createServerInfos(t)
		t.Cleanup(func() { deleteAllServerInfos(t) })

		// DeleteServerInfo should fail if name isn't provided.
		err := clt.DeleteServerInfo(ctx, "")
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))

		// Delete server info.
		err = clt.DeleteServerInfo(ctx, serverInfo1.GetName())
		require.NoError(t, err)

		// Expect server info not found.
		_, err = clt.GetServerInfo(ctx, serverInfo1.GetName())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))

		// Expect other server info still exists.
		si, err := clt.GetServerInfo(ctx, serverInfo2.GetName())
		require.NoError(t, err)
		requireResourcesEqual(t, serverInfo2, si)
	})
}

// TestSAMLIdPServiceProvidersCRUD tests SAMLIdPServiceProviders resource operations.
func TestSAMLIdPServiceProvidersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create two SAML IdP service providers.
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp1"),
			EntityID:         "sp1",
		})
	require.NoError(t, err)

	sp2, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp2",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp2"),
			EntityID:         "sp2",
		})
	require.NoError(t, err)

	// Initially we expect no service providers.
	listResp, nextKey, err := clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, listResp)

	// Create both service providers
	err = clt.CreateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	err = clt.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.NoError(t, err)

	// Fetch all service providers
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, listResp,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Update a service provider.
	sp1.SetEntityDescriptor(newEntityDescriptor("updated-sp1"))
	sp1.SetEntityID("updated-sp1")

	err = clt.UpdateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, listResp,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete a service provider.
	err = clt.DeleteSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp2}, listResp,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete a service provider that doesn't exist.
	err = clt.DeleteSAMLIdPServiceProvider(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Delete all service providers.
	err = clt.DeleteAllSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, listResp)
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	testCases := map[string]struct {
		resourceType   string
		createResource func(name string, clt *authclient.Client) error
	}{
		"DatabaseServers": {
			resourceType: types.KindDatabaseServer,
			createResource: func(name string, clt *authclient.Client) error {
				server, err := types.NewDatabaseServerV3(types.Metadata{
					Name: name,
				}, types.DatabaseServerSpecV3{
					Database: mustCreateDatabase(t, name, defaults.ProtocolPostgres, "localhost:5432"),
					Hostname: "localhost",
					HostID:   uuid.New().String(),
				})
				if err != nil {
					return err
				}

				_, err = clt.UpsertDatabaseServer(ctx, server)
				return err
			},
		},
		"ApplicationServers": {
			resourceType: types.KindAppServer,
			createResource: func(name string, clt *authclient.Client) error {
				app, err := types.NewAppV3(types.Metadata{
					Name: name,
				}, types.AppSpecV3{
					URI: "localhost",
				})
				if err != nil {
					return err
				}

				server, err := types.NewAppServerV3(types.Metadata{
					Name: name,
				}, types.AppServerSpecV3{
					Hostname: "localhost",
					HostID:   uuid.New().String(),
					App:      app,
				})
				if err != nil {
					return err
				}

				_, err = clt.UpsertApplicationServer(ctx, server)
				return err
			},
		},
		"KubeServer": {
			resourceType: types.KindKubeServer,
			createResource: func(name string, clt *authclient.Client) error {
				kube, err := types.NewKubernetesClusterV3(
					types.Metadata{
						Name:   name,
						Labels: map[string]string{"name": name},
					},
					types.KubernetesClusterSpecV3{},
				)
				if err != nil {
					return err
				}

				kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "_", "_")
				if err != nil {
					return err
				}
				_, err = clt.UpsertKubernetesServer(ctx, kubeServer)
				return err
			},
		},
		"Node": {
			resourceType: types.KindNode,
			createResource: func(name string, clt *authclient.Client) error {
				server, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{})
				if err != nil {
					return err
				}

				_, err = clt.UpsertNode(ctx, server)
				return err
			},
		},
		"WindowsDesktops": {
			resourceType: types.KindWindowsDesktop,
			createResource: func(name string, clt *authclient.Client) error {
				desktop, err := types.NewWindowsDesktopV3(name, nil,
					types.WindowsDesktopSpecV3{Addr: "_", HostID: "_"})
				if err != nil {
					return err
				}

				return clt.UpsertWindowsDesktop(ctx, desktop)
			},
		},
	}

	for name, test := range testCases {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: test.resourceType,
				Namespace:    apidefaults.Namespace,
				Limit:        100,
			})
			require.NoError(t, err)
			require.Empty(t, resp.Resources)
			require.Empty(t, resp.NextKey)

			// create two resources
			err = test.createResource("foo", clt)
			require.NoError(t, err)
			err = test.createResource("bar", clt)
			require.NoError(t, err)

			resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: test.resourceType,
				Namespace:    apidefaults.Namespace,
				Limit:        100,
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 2)
			require.Empty(t, resp.NextKey)
			require.Empty(t, resp.TotalCount)

			// ListResources should also work when called on auth directly
			resp, err = srv.Auth().ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: test.resourceType,
				Namespace:    apidefaults.Namespace,
				Limit:        100,
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 2)
			require.Empty(t, resp.NextKey)
			require.Empty(t, resp.TotalCount)

			// Test types.KindKubernetesCluster
			if test.resourceType == types.KindKubeServer {
				test.resourceType = types.KindKubernetesCluster
				resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.resourceType,
					Namespace:    apidefaults.Namespace,
					Limit:        100,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 2)
				require.Empty(t, resp.NextKey)
				require.Equal(t, 2, resp.TotalCount)
			} else {
				// Test listing with NeedTotalCount flag.
				resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType:   test.resourceType,
					Limit:          100,
					NeedTotalCount: true,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 2)
				require.Empty(t, resp.NextKey)
				require.Equal(t, 2, resp.TotalCount)
			}
		})
	}

	t.Run("InvalidResourceType", func(t *testing.T) {
		_, err := clt.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: "",
			Namespace:    apidefaults.Namespace,
			Limit:        100,
		})
		require.Error(t, err)
	})
}

func TestCustomRateLimiting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tests := []struct {
		name  string
		burst int
		fn    func(*authclient.Client) error
	}{
		{
			name: "RPC ChangeUserAuthentication",
			fn: func(clt *authclient.Client) error {
				_, err := clt.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{})
				return err
			},
		},
		{
			name:  "RPC CreateAuthenticateChallenge",
			burst: defaults.LimiterBurst,
			fn: func(clt *authclient.Client) error {
				_, err := clt.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{})
				return err
			},
		},
		{
			name: "RPC GetAccountRecoveryToken",
			fn: func(clt *authclient.Client) error {
				_, err := clt.GetAccountRecoveryToken(ctx, &proto.GetAccountRecoveryTokenRequest{})
				return err
			},
		},
		{
			name: "RPC StartAccountRecovery",
			fn: func(clt *authclient.Client) error {
				_, err := clt.StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{})
				return err
			},
		},
		{
			name: "RPC VerifyAccountRecovery",
			fn: func(clt *authclient.Client) error {
				_, err := clt.VerifyAccountRecovery(ctx, &proto.VerifyAccountRecoveryRequest{})
				return err
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Create new instance per test case, to troubleshoot which test case
			// specifically failed, otherwise multiple cases can fail from running
			// cases in parallel.
			srv := newTestTLSServer(t)
			clt, err := srv.NewClient(TestNop())
			require.NoError(t, err)

			var attempts int
			if test.burst == 0 {
				attempts = 10 // Good for most tests.
			} else {
				attempts = test.burst
			}

			for i := 0; i < attempts; i++ {
				err = test.fn(clt)
				require.False(t, trace.IsLimitExceeded(err), "got err = %v, want non-IsLimitExceeded", err)
			}

			err = test.fn(clt)
			require.True(t, trace.IsLimitExceeded(err), "got err = %v, want LimitExceeded", err)
		})
	}
}

type mockAuthorizer struct {
	ctx *authz.Context
	err error
}

func (a mockAuthorizer) Authorize(context.Context) (*authz.Context, error) {
	return a.ctx, a.err
}

type mockTraceClient struct {
	err   error
	spans []*otlptracev1.ResourceSpans
}

func (m mockTraceClient) Start(ctx context.Context) error {
	return nil
}

func (m mockTraceClient) Stop(ctx context.Context) error {
	return nil
}

func (m *mockTraceClient) UploadTraces(ctx context.Context, protoSpans []*otlptracev1.ResourceSpans) error {
	m.spans = protoSpans

	return m.err
}

func TestExport(t *testing.T) {
	t.Parallel()
	uploadErr := trace.AccessDenied("failed to upload")

	const user = "user"

	validateResource := func(forwardedFor string, resourceSpan *otlptracev1.ResourceSpans) {
		var forwarded []string
		for _, attribute := range resourceSpan.Resource.Attributes {
			if attribute.Key == forwardedTag {
				forwarded = append(forwarded, attribute.Value.GetStringValue())
			}
		}

		require.Len(t, forwarded, 1)

		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				for _, attribute := range span.Attributes {
					if attribute.Key == forwardedTag {
						forwarded = append(forwarded, attribute.Value.GetStringValue())
					}
				}
			}
		}

		require.Len(t, forwarded, 2)
		for _, value := range forwarded {
			require.Equal(t, forwardedFor, value)
		}
	}

	validateTaggedSpans := func(forwardedFor string) require.ValueAssertionFunc {
		return func(t require.TestingT, i interface{}, i2 ...interface{}) {
			require.NotEmpty(t, i)
			resourceSpans, ok := i.([]*otlptracev1.ResourceSpans)
			require.True(t, ok)

			for _, resourceSpan := range resourceSpans {
				if resourceSpan.Resource != nil {
					validateResource(forwardedFor, resourceSpan)
					return
				}

				for _, scopeSpan := range resourceSpan.ScopeSpans {
					for _, span := range scopeSpan.Spans {
						var foundForwardedTag bool
						for _, attribute := range span.Attributes {
							if attribute.Key == forwardedTag {
								require.False(t, foundForwardedTag)
								foundForwardedTag = true
								require.Equal(t, forwardedFor, attribute.Value.GetStringValue())
							}
						}
						require.True(t, foundForwardedTag)
					}
				}
			}
		}
	}

	testSpans := []*otlptracev1.ResourceSpans{
		{
			Resource: &otlpresourcev1.Resource{
				Attributes: []*otlpcommonv1.KeyValue{
					{
						Key: "test",
						Value: &otlpcommonv1.AnyValue{
							Value: &otlpcommonv1.AnyValue_IntValue{
								IntValue: 1,
							},
						},
					},
					{
						Key: "key",
						Value: &otlpcommonv1.AnyValue{
							Value: &otlpcommonv1.AnyValue_StringValue{
								StringValue: user,
							},
						},
					},
				},
			},
			ScopeSpans: []*otlptracev1.ScopeSpans{
				{
					Spans: []*otlptracev1.Span{
						{
							Name: "with-attributes",
							Attributes: []*otlpcommonv1.KeyValue{
								{
									Key: "test",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_IntValue{
											IntValue: 1,
										},
									},
								},
								{
									Key: "key",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_DoubleValue{
											DoubleValue: 5.0,
										},
									},
								},
							},
						},
						{
							Name:       "with-tag",
							Attributes: []*otlpcommonv1.KeyValue{{Key: forwardedTag, Value: &otlpcommonv1.AnyValue{Value: &otlpcommonv1.AnyValue_StringValue{StringValue: "test"}}}},
						},
						{
							Name: "no-attributes",
						},
					},
				},
			},
		},
		{
			ScopeSpans: []*otlptracev1.ScopeSpans{
				{
					Spans: []*otlptracev1.Span{
						{
							Name: "more-with-attributes",
							Attributes: []*otlpcommonv1.KeyValue{
								{
									Key: "test2",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_IntValue{
											IntValue: 11,
										},
									},
								},
								{
									Key: "key2",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_DoubleValue{
											DoubleValue: 15.0,
										},
									},
								},
							},
						},
						{
							Name: "already-tagged",
							Attributes: []*otlpcommonv1.KeyValue{
								{
									Key: forwardedTag,
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_StringValue{
											StringValue: user,
										},
									},
								},
								{
									Key: "key2",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_DoubleValue{
											DoubleValue: 15.0,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cases := []struct {
		name              string
		identity          TestIdentity
		errAssertion      require.ErrorAssertionFunc
		uploadedAssertion require.ValueAssertionFunc
		spans             []*otlptracev1.ResourceSpans
		authorizer        authz.Authorizer
		mockTraceClient   mockTraceClient
	}{
		{
			name:              "error when unauthorized",
			identity:          TestNop(),
			errAssertion:      require.Error,
			uploadedAssertion: require.Empty,
			spans:             make([]*otlptracev1.ResourceSpans, 1),
			authorizer:        &mockAuthorizer{err: trace.AccessDenied("unauthorized")},
		},
		{
			name:              "nop for empty spans",
			identity:          TestBuiltin(types.RoleNode),
			errAssertion:      require.NoError,
			uploadedAssertion: require.Empty,
		},
		{
			name:     "failure to forward spans",
			identity: TestBuiltin(types.RoleNode),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, trail.FromGRPC(trace.Unwrap(err)), uploadErr)
			},
			uploadedAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				require.Len(t, i, 1)
			},
			spans:           make([]*otlptracev1.ResourceSpans, 1),
			mockTraceClient: mockTraceClient{err: uploadErr},
		},
		{
			name:              "forwarded spans get tagged for system roles",
			identity:          TestBuiltin(types.RoleProxy),
			errAssertion:      require.NoError,
			spans:             testSpans,
			uploadedAssertion: validateTaggedSpans(fmt.Sprintf("%s.localhost:%s", types.RoleProxy, types.RoleProxy)),
		},
		{
			name:              "forwarded spans get tagged for users",
			identity:          TestUser(user),
			errAssertion:      require.NoError,
			spans:             testSpans,
			uploadedAssertion: validateTaggedSpans(user),
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			as, err := NewTestAuthServer(TestAuthServerConfig{
				Dir:         t.TempDir(),
				Clock:       clockwork.NewFakeClock(),
				TraceClient: &tt.mockTraceClient,
			})
			require.NoError(t, err)

			srv, err := as.NewTestTLSServer()
			require.NoError(t, err)

			t.Cleanup(func() { require.NoError(t, srv.Close()) })

			// Create a fake user.
			_, _, err = CreateUserAndRole(srv.Auth(), user, []string{"role"}, nil)
			require.NoError(t, err)

			// Setup the server
			if tt.authorizer != nil {
				srv.TLSServer.grpcServer.Authorizer = tt.authorizer
				require.NoError(t, err)
			}

			// Get a client for the test identity
			clt, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			// create a tracing client and forward some traces
			traceClt := tracing.NewClient(clt.APIClient.GetConnection())
			t.Cleanup(func() { require.NoError(t, traceClt.Close()) })
			require.NoError(t, traceClt.Start(ctx))

			tt.errAssertion(t, traceClt.UploadTraces(ctx, tt.spans))
			tt.uploadedAssertion(t, tt.mockTraceClient.spans)
		})
	}
}

// TestSAMLValidation tests that SAML validation does not perform an HTTP
// request if the calling user does not have permissions to create or update
// a SAML connector.
func TestSAMLValidation(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.SAML: {Enabled: true},
			},
		},
	})

	// minimal entity_descriptor to pass validation. not actually valid
	const minimalEntityDescriptor = `
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://example.com">
  <md:IDPSSODescriptor>
    <md:SingleSignOnService Location="http://example.com" />
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`

	allowSAMLUpsert := types.RoleConditions{
		Rules: []types.Rule{{
			Resources: []string{types.KindSAML},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		}},
	}

	testCases := []struct {
		desc               string
		allow              types.RoleConditions
		entityDescriptor   string
		entityServerCalled bool
		assertErr          func(error) bool
	}{
		{
			desc:               "access denied",
			allow:              types.RoleConditions{},
			entityServerCalled: false,
			assertErr:          trace.IsAccessDenied,
		},
		{
			desc:               "validation failure",
			allow:              allowSAMLUpsert,
			entityDescriptor:   "", // validation fails with no issuer
			entityServerCalled: true,
			assertErr:          trace.IsBadParameter,
		},
		{
			desc:               "access permitted",
			allow:              allowSAMLUpsert,
			entityDescriptor:   minimalEntityDescriptor,
			entityServerCalled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			server := newTestTLSServer(t)
			// Create an http server to serve the entity descriptor url
			entityServerCalled := false
			entityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				entityServerCalled = true
				_, err := w.Write([]byte(tc.entityDescriptor))
				require.NoError(t, err)
			}))

			role, err := CreateRole(ctx, server.Auth(), "test_role", types.RoleSpecV6{Allow: tc.allow})
			require.NoError(t, err)
			user, err := CreateUser(ctx, server.Auth(), "test_user", role)
			require.NoError(t, err)

			connector, err := types.NewSAMLConnector("test_connector", types.SAMLConnectorSpecV2{
				AssertionConsumerService: "http://localhost:65535/acs", // not called
				EntityDescriptorURL:      entityServer.URL,
				AttributesToRoles: []types.AttributeMapping{
					// not used. can be any name, value but role must exist
					{Name: "groups", Value: "admin", Roles: []string{role.GetName()}},
				},
			})
			require.NoError(t, err)

			client, err := server.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			_, err = client.UpsertSAMLConnector(ctx, connector)

			if tc.assertErr != nil {
				require.Error(t, err)
				require.True(t, tc.assertErr(err), "UpsertSAMLConnector error type mismatch. got: %T", trace.Unwrap(err))
			} else {
				require.NoError(t, err)
			}
			if tc.entityServerCalled {
				require.True(t, entityServerCalled, "entity_descriptor_url was not called")
			} else {
				require.False(t, entityServerCalled, "entity_descriptor_url was called")
			}
		})
	}
}

func newEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `
<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`

func TestGRPCServer_GetInstallers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := newTestTLSServer(t)
	grpc := server.TLSServer.grpcServer

	user := TestAdmin()
	ctx = authz.ContextWithUser(ctx, user.I)

	tests := []struct {
		name               string
		inputInstallers    map[string]string
		hasAgentRollout    bool
		expectedInstallers map[string]string
	}{
		{
			name: "default installers only",
			expectedInstallers: map[string]string{
				types.DefaultInstallerScriptName:        installer.LegacyDefaultInstaller.GetScript(),
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
		{
			name:            "new default installers",
			hasAgentRollout: true,
			expectedInstallers: map[string]string{
				types.DefaultInstallerScriptName:        installer.NewDefaultInstaller.GetScript(),
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
		{
			name: "default and custom installers",
			inputInstallers: map[string]string{
				"my-custom-installer": "echo test",
			},
			expectedInstallers: map[string]string{
				"my-custom-installer":                   "echo test",
				types.DefaultInstallerScriptName:        installer.LegacyDefaultInstaller.GetScript(),
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
		{
			name: "override default installer",
			inputInstallers: map[string]string{
				installers.InstallerScriptName: "echo test",
			},
			expectedInstallers: map[string]string{
				installers.InstallerScriptName:          "echo test",
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				_, err := grpc.DeleteAllInstallers(ctx, &emptypb.Empty{})
				require.NoError(t, err)
			})

			if tc.hasAgentRollout {
				rollout, err := autoupdate.NewAutoUpdateAgentRollout(
					&autoupdatev1pb.AutoUpdateAgentRolloutSpec{
						StartVersion:              "1.2.3",
						TargetVersion:             "1.2.4",
						Schedule:                  autoupdate.AgentsScheduleImmediate,
						AutoupdateMode:            autoupdate.AgentsUpdateModeEnabled,
						Strategy:                  autoupdate.AgentsStrategyTimeBased,
						MaintenanceWindowDuration: durationpb.New(1 * time.Hour),
					})
				require.NoError(t, err)
				_, err = grpc.AuthServer.CreateAutoUpdateAgentRollout(ctx, rollout)
				require.NoError(t, err)

				t.Cleanup(func() {
					assert.NoError(t, grpc.AuthServer.DeleteAutoUpdateAgentRollout(ctx))
				})
			}

			for name, script := range tc.inputInstallers {
				installer, err := types.NewInstallerV1(name, script)
				require.NoError(t, err)
				_, err = grpc.SetInstaller(ctx, installer)
				require.NoError(t, err)
			}

			outputInstallerList, err := grpc.GetInstallers(ctx, &emptypb.Empty{})
			require.NoError(t, err)
			outputInstallers := make(map[string]string, len(tc.expectedInstallers))
			for _, installer := range outputInstallerList.Installers {
				outputInstallers[installer.GetName()] = installer.GetScript()
			}

			require.Equal(t, tc.expectedInstallers, outputInstallers)
		})
	}
}

func TestRoleVersions(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	newRole := func(name string, version string, spec types.RoleSpecV6) types.Role {
		role, err := types.NewRoleWithVersion(name, version, spec)
		meta := role.GetMetadata()
		role.SetMetadata(meta)
		require.NoError(t, err)
		return role
	}

	enabledRole := newRole("test_role_enabled", types.V7, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindRole, services.RW()),
			},
		},
		Options: types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{
					Enabled: types.NewBoolOption(true),
				},
				Local: &types.SSHLocalPortForwarding{
					Enabled: types.NewBoolOption(true),
				},
			},
		},
	})

	disabledRole := newRole("test_role_disabled", types.V7, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindRole, services.RW()),
			},
		},
		Options: types.RoleOptions{
			SSHPortForwarding: &types.SSHPortForwarding{
				Remote: &types.SSHRemotePortForwarding{
					Enabled: types.NewBoolOption(false),
				},
				Local: &types.SSHLocalPortForwarding{
					Enabled: types.NewBoolOption(false),
				},
			},
		},
	})

	undefinedRole := newRole("test_role_implicit", types.V7, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindRole, services.RW()),
			},
		},
	})

	user, err := CreateUser(context.Background(), srv.Auth(), "user", enabledRole, disabledRole, undefinedRole)
	require.NoError(t, err)

	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, tc := range []struct {
		desc             string
		clientVersions   []string
		expectError      bool
		inputRole        types.Role
		expectedRole     types.Role
		expectDowngraded bool
	}{
		{
			desc: "up to date - enabled",
			clientVersions: []string{
				"17.1.0", "17.1.0-dev", "18.0.0-dev", "19.0.0", "",
			},
			inputRole:    enabledRole,
			expectedRole: enabledRole,
		},
		{
			desc: "up to date - disabled",
			clientVersions: []string{
				"17.1.0", "17.1.0-dev", "18.0.0-dev", "19.0.0", "",
			},
			inputRole:    disabledRole,
			expectedRole: disabledRole,
		},
		{
			desc: "up to date - undefined",
			clientVersions: []string{
				"17.1.0", "17.1.0-dev", "18.0.0-dev", "19.0.0", "",
			},
			inputRole:    undefinedRole,
			expectedRole: undefinedRole,
		},
		{
			desc: "downgrade SSH access control granularity - enabled",
			clientVersions: []string{
				"17.0.0",
			},
			inputRole: enabledRole,
			expectedRole: newRole(enabledRole.GetName(), types.V7, types.RoleSpecV6{
				Allow: types.RoleConditions{
					Rules: []types.Rule{
						types.NewRule(types.KindRole, services.RW()),
					},
				},
				Options: types.RoleOptions{
					PortForwarding:    types.NewBoolOption(true),
					SSHPortForwarding: enabledRole.GetOptions().SSHPortForwarding,
				},
			}),
			expectDowngraded: true,
		},
		{
			desc: "downgrade SSH access control granularity - disabled",
			clientVersions: []string{
				"17.0.0",
			},
			inputRole: disabledRole,
			expectedRole: newRole(disabledRole.GetName(), types.V7, types.RoleSpecV6{
				Allow: types.RoleConditions{
					Rules: []types.Rule{
						types.NewRule(types.KindRole, services.RW()),
					},
				},
				Options: types.RoleOptions{
					PortForwarding:    types.NewBoolOption(false),
					SSHPortForwarding: disabledRole.GetOptions().SSHPortForwarding,
				},
			}),
			expectDowngraded: true,
		},
		{
			desc: "downgrade SSH access control granularity - undefined",
			clientVersions: []string{
				"17.0.0",
			},
			inputRole:        undefinedRole,
			expectedRole:     undefinedRole,
			expectDowngraded: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			for _, clientVersion := range tc.clientVersions {
				t.Run(clientVersion, func(t *testing.T) {
					// setup client metadata
					ctx := context.Background()
					if clientVersion == "" {
						ctx = context.WithValue(ctx, metadata.DisableInterceptors{}, struct{}{})
					} else {
						ctx = metadata.AddMetadataToContext(ctx, map[string]string{
							metadata.VersionKey: clientVersion,
						})
					}

					checkRole := func(gotRole types.Role) {
						t.Helper()
						if tc.expectError {
							return
						}
						require.Empty(t, cmp.Diff(tc.expectedRole, gotRole,
							cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Labels")))
						// The downgraded label value won't match exactly because it
						// includes the client version, so just check it's not empty
						// and ignore it in the role diff.
						if tc.expectDowngraded {
							require.NotEmpty(t, gotRole.GetMetadata().Labels[types.TeleportDowngradedLabel])
						}
					}
					checkErr := func(err error) {
						t.Helper()
						if tc.expectError {
							require.Error(t, err)
						} else {
							require.NoError(t, err)
						}
					}

					// Test GetRole
					gotRole, err := client.GetRole(ctx, tc.inputRole.GetName())
					checkErr(err)
					checkRole(gotRole)

					// Test GetRoles
					gotRoles, err := client.GetRoles(ctx)
					checkErr(err)
					if !tc.expectError {
						foundTestRole := false
						for _, gotRole := range gotRoles {
							if gotRole.GetName() != tc.inputRole.GetName() {
								continue
							}
							checkRole(gotRole)
							foundTestRole = true
							break
						}
						require.True(t, foundTestRole, "GetRoles result does not include expected role")
					}

					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()

					// Test WatchEvents
					watcher, err := client.NewWatcher(ctx, types.Watch{Name: "roles", Kinds: []types.WatchKind{{Kind: types.KindRole}}})
					require.NoError(t, err)
					defer watcher.Close()

					// Swallow the init event
					e := <-watcher.Events()
					require.Equal(t, types.OpInit, e.Type)

					// Re-upsert the role so that the watcher sees it, do this
					// on the auth server directly to avoid the
					// TeleportDowngradedLabel check in ServerWithRoles
					tc.inputRole, err = srv.Auth().UpsertRole(ctx, tc.inputRole)
					require.NoError(t, err)

					gotRole, err = func() (types.Role, error) {
						for {
							select {
							case <-watcher.Done():
								return nil, watcher.Error()
							case e := <-watcher.Events():
								if gotRole, ok := e.Resource.(types.Role); ok && gotRole.GetName() == tc.inputRole.GetName() {
									return gotRole, nil
								}
							}
						}
					}()
					checkErr(err)
					checkRole(gotRole)

					if !tc.expectError {
						// Try to re-upsert the role we got. If it was
						// downgraded, it should be rejected due to the
						// TeleportDowngradedLabel
						_, err = client.UpsertRole(ctx, gotRole)
						if tc.expectDowngraded {
							require.Error(t, err)
						} else {
							require.NoError(t, err)
						}
					}
				})
			}
		})
	}
}

func TestUpsertApplicationServerOrigin(t *testing.T) {
	t.Parallel()

	parentCtx := context.Background()
	server := newTestTLSServer(t)

	admin := TestAdmin()

	client, err := server.NewClient(admin)
	require.NoError(t, err)

	// Dynamic origin should work for admin role.
	app, err := types.NewAppV3(types.Metadata{
		Name:   "app1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)
	appServer, err := types.NewAppServerV3FromApp(app, "localhost", "123456")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(parentCtx, admin.I)
	_, err = client.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	// Okta origin should not work for admin role.
	app.SetOrigin(types.OriginOkta)
	appServer, err = types.NewAppServerV3FromApp(app, "localhost", "123456")
	require.NoError(t, err)

	ctx = authz.ContextWithUser(parentCtx, admin.I)
	_, err = client.UpsertApplicationServer(ctx, appServer)
	require.True(t, trace.IsBadParameter(err), "trace.IsBadParameter failed: err=%v (%T)", err, trace.Unwrap(err))
	require.ErrorContains(t, err, "only the Okta role can create app servers and apps with an Okta origin")

	// Okta origin should not work with instance and node roles.
	client, err = server.NewClient(TestIdentity{
		I: authz.BuiltinRole{
			Role: types.RoleInstance,
			AdditionalSystemRoles: []types.SystemRole{
				types.RoleNode,
			},
			Username: server.ClusterName(),
		},
	})
	require.NoError(t, err)

	ctx = authz.ContextWithUser(parentCtx, admin.I)
	_, err = client.UpsertApplicationServer(ctx, appServer)
	require.True(t, trace.IsBadParameter(err), "trace.IsBadParameter failed: err=%v (%T)", err, trace.Unwrap(err))
	require.ErrorContains(t, err, "only the Okta role can create app servers and apps with an Okta origin")

	// Okta origin should work with Okta role in role field.
	node := TestIdentity{
		I: authz.BuiltinRole{
			Role: types.RoleOkta,
			AdditionalSystemRoles: []types.SystemRole{
				types.RoleNode,
			},
			Username: server.ClusterName(),
		},
	}
	client, err = server.NewClient(node)
	require.NoError(t, err)

	ctx = authz.ContextWithUser(parentCtx, node.I)
	_, err = client.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	// Okta origin should work with Okta role in additional system roles.
	node = TestIdentity{
		I: authz.BuiltinRole{
			Role: types.RoleInstance,
			AdditionalSystemRoles: []types.SystemRole{
				types.RoleNode,
				types.RoleOkta,
			},
			Username: server.ClusterName(),
		},
	}
	client, err = server.NewClient(node)
	require.NoError(t, err)

	ctx = authz.ContextWithUser(parentCtx, node.I)
	_, err = client.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)
}

func TestGetAccessGraphConfig(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
			},
		},
	})
	server := newTestTLSServer(t,
		withAccessGraphConfig(AccessGraphConfig{
			Enabled: true,
			CA:      []byte("ca"),
			Address: "addr",
		}),
	)
	user, _, err := CreateUserAndRole(server.Auth(), "test", []string{"role"}, nil)
	require.NoError(t, err)
	positiveResponse := &clusterconfigpb.AccessGraphConfig{
		Enabled:           true,
		Ca:                []byte("ca"),
		Address:           "addr",
		SecretsScanConfig: &clusterconfigpb.AccessGraphSecretsScanConfiguration{},
	}

	tests := []struct {
		desc      string
		identity  authz.IdentityGetter
		assertErr require.ErrorAssertionFunc
		expected  *clusterconfigpb.AccessGraphConfig
	}{
		{
			desc: "users can't pull the access graph config",
			identity: authz.LocalUser{
				Username: user.GetName(),
			},
			assertErr: require.Error,
		},
		{
			desc: "proxy can pull access graph config",
			identity: authz.BuiltinRole{
				Role:     types.RoleProxy,
				Username: server.ClusterName(),
			},
			assertErr: require.NoError,
			expected:  positiveResponse,
		},
		{
			desc: "discovery can pull access graph config",
			identity: authz.BuiltinRole{
				Role:     types.RoleDiscovery,
				Username: server.ClusterName(),
			},
			assertErr: require.NoError,
			expected:  positiveResponse,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			client, err := server.NewClient(TestIdentity{
				I: test.identity,
			})
			require.NoError(t, err)
			rsp, err := client.GetClusterAccessGraphConfig(context.Background())
			test.assertErr(t, err)
			require.Empty(t, cmp.Diff(test.expected, rsp, protocmp.Transform()))
		})
	}
}

func TestCreateAuditStreamLimit(t *testing.T) {
	const N = 5
	t.Setenv("TELEPORT_UNSTABLE_CREATEAUDITSTREAM_INFLIGHT_LIMIT", fmt.Sprintf("%d", N))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := newTestTLSServer(t)
	clt, err := server.NewClient(TestServerID(types.RoleNode, uuid.NewString()))
	require.NoError(t, err)

	// HACK(espadolini): we're piggybacking on the prometheus counter which
	// can't change while this test is running (we set an envvar, so we can't be
	// running in parallel with other tests) but it's still pretty awful, and
	// it'd be much better to actually check that the streams were accepted by
	// the server; unfortunately, the CreateAuditStream stream doesn't actually
	// send anything back unless there's a real upload going on, and the test
	// server uses a discard emitter which never ends up sending anything
	getAcceptedTotal := func() int {
		var m prom_client_model.Metric
		require.NoError(t, createAuditStreamAcceptedTotalMetric.Write(&m))
		return int(m.Counter.GetValue())
	}
	currentAcceptedTotal := getAcceptedTotal()

	for i := 0; i < N; i++ {
		stream, err := clt.CreateAuditStream(ctx, session.NewID())
		require.NoError(t, err)
		t.Cleanup(func() { stream.Close(ctx) })
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.EqualValues(t, currentAcceptedTotal+N, getAcceptedTotal())
	}, time.Second, 100*time.Millisecond)

	ac := proto.NewAuthServiceClient(clt.APIClient.GetConnection())
	stream, err := ac.CreateAuditStream(ctx)
	require.NoError(t, err)
	_, err = stream.Recv()
	require.ErrorAs(t, err, new(*trace.ConnectionProblemError))
}
