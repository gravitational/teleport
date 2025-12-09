// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mfav1_test

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authtest"
	mfav1impl "github.com/gravitational/teleport/lib/auth/mfa/mfav1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestCreateSessionChallenge_WebAuthn tests the CreateSessionChallenge method for WebAuthn challenges.
func TestCreateSessionChallenge_WebAuthn(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	},
		nil,
	)
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		}},
	)

	_, err = authtest.RegisterTestDevice(
		ctx,
		authServer.Auth(),
		"webauthn-device",
		proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		nil,
	)
	require.NoError(t, err)

	resp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
		},
	)
	require.NoError(t, err)

	// Verify a WebAuthn challenge was returned.
	require.NotNil(t, resp)
	require.NotNil(t, resp.MfaChallenge)
	require.NotNil(t, resp.MfaChallenge.WebauthnChallenge)
	require.NotEmpty(t, resp.MfaChallenge.WebauthnChallenge.PublicKey.Challenge)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.CreateMFAAuthChallengeEvent, event.GetType())
	require.Equal(t, events.CreateMFAAuthChallengeCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.CreateMFAAuthChallenge)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
}

// TestCreateSessionChallenge_SSO tests the CreateSessionChallenge method for SSO challenges.
func TestCreateSessionChallenge_SSO(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
					types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	},
		[]*types.MFADevice{
			{
				Device: &types.MFADevice_Sso{
					Sso: &types.SSOMFADevice{
						DisplayName:   "test-display-name",
						ConnectorId:   "test-device-connector-id",
						ConnectorType: constants.SAML,
					},
				},
			},
		},
	)
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		}},
	)

	resp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
			SsoClientRedirectUrl: "https://sso/redirect",
			ProxyAddressForSso:   "proxy.example.com",
		},
	)
	require.NoError(t, err)

	// Verify SSO challenge was returned.
	require.NotNil(t, resp)
	require.NotNil(t, resp.MfaChallenge)
	require.NotNil(t, resp.MfaChallenge.SsoChallenge)
	require.Equal(t, "test-display-name", resp.MfaChallenge.SsoChallenge.Device.DisplayName)
	require.Equal(t, "test-device-connector-id", resp.MfaChallenge.SsoChallenge.Device.ConnectorId)
	require.Equal(t, constants.SAML, resp.MfaChallenge.SsoChallenge.Device.ConnectorType)
	require.Equal(t, "https://sso/redirect", resp.MfaChallenge.SsoChallenge.RedirectUrl)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.CreateMFAAuthChallengeEvent, event.GetType())
	require.Equal(t, events.CreateMFAAuthChallengeCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.CreateMFAAuthChallenge)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
}

// TestCreateSessionChallenge_InvalidRequest tests that CreateSessionChallenge returns errors for invalid requests.
func TestCreateSessionChallenge_InvalidRequest(t *testing.T) {
	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	for _, testCase := range []struct {
		name string
		req  *mfav1.CreateSessionChallengeRequest
	}{
		{
			name: "nil request",
			req:  nil,
		},
		{
			name: "missing payload",
			req:  &mfav1.CreateSessionChallengeRequest{Payload: nil},
		},
		{
			name: "missing SshSessionId in payload",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{Payload: nil},
			},
		},
		{
			name: "empty SshSessionId in payload",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte{},
					},
				},
			},
		},
		{
			name: "SSO challenge missing SsoClientRedirectUrl",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte("test-session-id"),
					},
				},
				SsoClientRedirectUrl: "", // missing
				ProxyAddressForSso:   "proxy.example.com",
			},
		},
		{
			name: "SSO challenge missing ProxyAddressForSso",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte("test-session-id"),
					},
				},
				SsoClientRedirectUrl: "https://client/redirect",
				ProxyAddressForSso:   "", // missing
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp, err := service.CreateSessionChallenge(t.Context(), testCase.req)
			require.True(t, trace.IsBadParameter(err))
			require.Nil(t, resp)
		})
	}
}

// TestCreateSessionChallenge_NoMFADevices tests the CreateSessionChallenge method when no MFA devices are registered.
func TestCreateSessionChallenge_NoMFADevices(t *testing.T) {
	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	},
		[]*types.MFADevice{
			// No devices.
		},
	)
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		}},
	)

	_, err = service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
		},
	)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "has no registered MFA devices")
}

// TestValidateSessionChallenge tests the ValidateSessionChallenge method for WebAuthn challenges.
func TestValidateSessionChallenge_WebAuthn(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
					types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	},
		nil,
	)
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		},
	})

	// Register a WebAuthn device for the user.
	device, err := authtest.RegisterTestDevice(
		ctx,
		authServer.Auth(),
		"webauthn-device",
		proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		nil,
	)
	require.NoError(t, err)

	challengeResp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, challengeResp)
	require.NotNil(t, challengeResp.MfaChallenge)
	require.NotNil(t, challengeResp.MfaChallenge.WebauthnChallenge)

	challenge := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: challengeResp.MfaChallenge.WebauthnChallenge,
	}

	// Simulate solving the WebAuthn challenge so it is stored in the backend and can be retrieved for validation.
	mfaResp, err := device.SolveAuthn(challenge)
	require.NoError(t, err)

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Response: &mfav1.AuthenticateResponse_Webauthn{
					Webauthn: mfaResp.GetWebauthn(),
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, validateResp)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, e.MFADevice.DeviceName, device.MFA.Metadata.GetName())
	require.Equal(t, e.MFADevice.DeviceID, device.MFA.Id)
	require.Equal(t, e.MFADevice.DeviceType, device.MFA.MFAType())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
}

// TestValidateSessionChallenge tests the ValidateSessionChallenge method for SSO challenges.
func TestValidateSessionChallenge_SSO(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
					types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	},
		[]*types.MFADevice{
			{
				Metadata: types.Metadata{
					Name: "sso-device",
				},
				Device: &types.MFADevice_Sso{
					Sso: &types.SSOMFADevice{
						DisplayName:   "test-display-name",
						ConnectorId:   "test-device-connector-id",
						ConnectorType: constants.SAML,
					},
				},
			},
		},
	)
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user")
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{
		Identity: tlsca.Identity{
			Username: user.GetName(),
		},
	})

	resp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Response: &mfav1.AuthenticateResponse_Sso{
					Sso: &mfav1.SSOChallengeResponse{
						RequestId: "test-request-id",
						Token:     "valid-token",
					},
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
}

// TestValidateSessionChallenge_InvalidRequest tests that ValidateSessionChallenge returns errors for invalid requests.
func TestValidateSessionChallenge_InvalidRequest(t *testing.T) {
	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
	})
	require.NoError(t, err)

	for _, testCase := range []struct {
		name string
		req  *mfav1.ValidateSessionChallengeRequest
	}{
		{
			name: "nil request",
			req:  nil,
		},
		{
			name: "missing MfaResponse",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: nil,
			},
		},
		{
			name: "missing Response",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: &mfav1.AuthenticateResponse{
					Response: nil,
				},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp, err := service.ValidateSessionChallenge(t.Context(), testCase.req)
			require.Nil(t, resp)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}
