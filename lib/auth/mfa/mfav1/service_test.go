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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authtest"
	mfav1impl "github.com/gravitational/teleport/lib/auth/mfa/mfav1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestCreateValidateSessionChallenge_Webauthn(t *testing.T) {
	t.Parallel()

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
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	// Register a Webauthn device for the user.
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

	// Verify a Webauthn challenge was returned.
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetWebauthnChallenge(), "WebauthnChallenge must not be nil")

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.CreateMFAAuthChallengeEvent, event.GetType())
	require.Equal(t, events.CreateMFAAuthChallengeCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	createEvent, ok := event.(*apievents.CreateMFAAuthChallenge)
	require.True(t, ok)
	require.Equal(t, user.GetName(), createEvent.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, createEvent.FlowType)

	challenge := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: challengeResp.MfaChallenge.WebauthnChallenge,
	}

	mfaResp, err := device.SolveAuthn(challenge)
	require.NoError(t, err)

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Name: challengeResp.MfaChallenge.Name,
				Response: &mfav1.AuthenticateResponse_Webauthn{
					Webauthn: mfaResp.GetWebauthn(),
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, validateResp)

	// Verify emitted event.
	event = emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	validateEvent, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), validateEvent.GetUser())
	require.Equal(t, validateEvent.MFADevice.DeviceName, device.MFA.Metadata.GetName())
	require.Equal(t, validateEvent.MFADevice.DeviceID, device.MFA.Id)
	require.Equal(t, validateEvent.MFADevice.DeviceType, device.MFA.MFAType())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, validateEvent.FlowType)

	// Verify stored ValidatedMFAChallenge.
	gotChallenge, err := authServer.Auth().MFAService.GetValidatedMFAChallenge(
		t.Context(),
		user.GetName(),
		challengeResp.GetMfaChallenge().GetName(),
	)
	require.NoError(t, err)

	wantedChallenge := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name: challengeResp.GetMfaChallenge().GetName(),
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
			SourceCluster: "test-cluster",
			TargetCluster: "test-cluster",
		},
	}

	diff := cmp.Diff(
		wantedChallenge,
		gotChallenge,
		cmp.FilterPath(
			// Ignore expiration time in comparison.
			func(p cmp.Path) bool {
				return p.String() == "Metadata.Expires"
			},
			cmp.Ignore(),
		),
	)
	require.Empty(t, diff, "GetValidatedMFAChallenge(%s, %s) mismatch (-want +got):\n%s", user.GetName(), challengeResp.GetMfaChallenge().GetName(), diff)
}

func TestCreateValidateSessionChallenge_SSO(t *testing.T) {
	t.Parallel()

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
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	challengeResp, err := service.CreateSessionChallenge(
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
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetSsoChallenge(), "SSOChallenge must not be nil")
	require.Equal(t, "test-display-name", challengeResp.GetMfaChallenge().GetSsoChallenge().GetDevice().DisplayName)
	require.Equal(t, "test-device-connector-id", challengeResp.GetMfaChallenge().GetSsoChallenge().GetDevice().ConnectorId)
	require.Equal(t, constants.SAML, challengeResp.GetMfaChallenge().GetSsoChallenge().GetDevice().ConnectorType)
	require.Equal(t, "https://sso/redirect", challengeResp.GetMfaChallenge().GetSsoChallenge().GetRedirectUrl())

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.CreateMFAAuthChallengeEvent, event.GetType())
	require.Equal(t, events.CreateMFAAuthChallengeCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	createEvent, ok := event.(*apievents.CreateMFAAuthChallenge)
	require.True(t, ok)
	require.Equal(t, user.GetName(), createEvent.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, createEvent.FlowType)

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Name: challengeResp.GetMfaChallenge().GetName(),
				Response: &mfav1.AuthenticateResponse_Sso{
					Sso: &mfav1.SSOChallengeResponse{
						RequestId: challengeResp.GetMfaChallenge().GetSsoChallenge().GetRequestId(),
					},
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, validateResp)

	// Verify emitted event.
	event = emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	validateEvent, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), validateEvent.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, validateEvent.FlowType)

	// Verify stored ValidatedMFAChallenge.
	gotChallenge, err := authServer.Auth().MFAService.GetValidatedMFAChallenge(
		t.Context(),
		user.GetName(),
		challengeResp.GetMfaChallenge().GetName(),
	)
	require.NoError(t, err)

	wantedChallenge := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name: challengeResp.GetMfaChallenge().GetName(),
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("test-session-id"),
				},
			},
			SourceCluster: "test-cluster",
			TargetCluster: "test-cluster",
		},
	}

	diff := cmp.Diff(
		wantedChallenge,
		gotChallenge,
		cmp.FilterPath(
			// Ignore expiration time in comparison.
			func(p cmp.Path) bool {
				return p.String() == "Metadata.Expires"
			},
			cmp.Ignore(),
		),
	)
	require.Empty(t, diff, "GetValidatedMFAChallenge(%s, %s) mismatch (-want +got):\n%s", user.GetName(), challengeResp.GetMfaChallenge().GetName(), diff)
}

func TestCreateSessionChallenge_NonLocalUserDenied(t *testing.T) {
	t.Parallel()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	ctx := t.Context()

	// Use a context with a system user.
	ctx = authz.ContextWithUser(ctx, authtest.TestBuiltin(types.RoleProxy).I)

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
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "only local or remote users can create MFA session challenges")
}

func TestCreateSessionChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			ClusterName: "test-cluster",
			Dir:         t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	for _, testCase := range []struct {
		name          string
		req           *mfav1.CreateSessionChallengeRequest
		expectedError string
	}{
		{
			name:          "missing payload",
			req:           &mfav1.CreateSessionChallengeRequest{Payload: nil},
			expectedError: "missing CreateSessionChallengeRequest payload",
		},
		{
			name: "missing SshSessionId in payload",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{Payload: nil},
			},
			expectedError: "empty SshSessionId in payload",
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
			expectedError: "empty SshSessionId in payload",
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
			expectedError: "missing SsoClientRedirectUrl for SSO challenge",
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
			expectedError: "missing ProxyAddressForSso for SSO challenge",
		},
		{
			name: "target cluster specified but does not exist",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte("test-session-id"),
					},
				},
				TargetCluster: "non-existent-cluster", // does not exist
			},
			expectedError: `cluster "non-existent-cluster" does not exist`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp, err := service.CreateSessionChallenge(ctx, testCase.req)
			require.True(t, trace.IsBadParameter(err))
			require.ErrorContains(t, err, testCase.expectedError)
			require.Nil(t, resp)
		})
	}
}

func TestCreateSessionChallenge_NoMFADevices(t *testing.T) {
	t.Parallel()

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
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

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

func TestValidateSessionChallenge_NonLocalUserDenied(t *testing.T) {
	t.Parallel()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	ctx := t.Context()

	// Use a context with a non-local user (e.g., a builtin role).
	ctx = authz.ContextWithUser(ctx, authtest.TestBuiltin(types.RoleProxy).I)

	_, err = service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Response: &mfav1.AuthenticateResponse_Webauthn{
					Webauthn: nil, // minimal, not relevant for this test
				},
			},
		},
	)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "only local or remote users can validate MFA session challenges")
}

func TestValidateSessionChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer.Auth(),
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	for _, testCase := range []struct {
		name string
		req  *mfav1.ValidateSessionChallengeRequest
	}{
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
			resp, err := service.ValidateSessionChallenge(ctx, testCase.req)
			require.Nil(t, resp)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

func TestValidateSessionChallenge_WebauthnFailedValidation(t *testing.T) {
	t.Parallel()

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
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	// Register a Webauthn device for the user.
	_, err = authtest.RegisterTestDevice(
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
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetWebauthnChallenge(), "WebauthnChallenge must not be nil")

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Name: challengeResp.GetMfaChallenge().GetName(),
				Response: &mfav1.AuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{
						Type: "invalid",
					},
				},
			},
		},
	)
	require.Error(t, err)
	require.Nil(t, validateResp)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseFailureCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
	require.False(t, e.Success)
}

func TestValidateSessionChallenge_SSOFailedValidation(t *testing.T) {
	t.Parallel()

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
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	challengeResp, err := service.CreateSessionChallenge(
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
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetSsoChallenge(), "SSOChallenge must not be nil")

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Name: challengeResp.GetMfaChallenge().GetName(),
				Response: &mfav1.AuthenticateResponse_Sso{
					Sso: &mfav1.SSOChallengeResponse{
						RequestId: "invalid-request-id-to-fail-validation",
					},
				},
			},
		},
	)
	require.Error(t, err)
	require.Nil(t, validateResp)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseFailureCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
	require.False(t, e.Success)
}

func TestValidateSessionChallenge_WebauthnFailedStorage(t *testing.T) {
	t.Parallel()

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

	mfaService := &mockMFAService{ReturnError: errors.New("MOCKED TEST ERROR FROM STORAGE LAYER")}

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    mfaService,
	})
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	// Register a Webauthn device for the user.
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
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetWebauthnChallenge(), "WebauthnChallenge must not be nil")

	challenge := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: challengeResp.MfaChallenge.WebauthnChallenge,
	}

	mfaResp, err := device.SolveAuthn(challenge)
	require.NoError(t, err)

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: &mfav1.AuthenticateResponse{
				Name: challengeResp.MfaChallenge.Name,
				Response: &mfav1.AuthenticateResponse_Webauthn{
					Webauthn: mfaResp.GetWebauthn(),
				},
			},
		},
	)
	require.ErrorContains(t, err, "MOCKED TEST ERROR FROM STORAGE LAYER")
	require.Nil(t, validateResp)

	// Verify emitted event.
	event := emitter.LastEvent()
	require.Equal(t, events.ValidateMFAAuthResponseEvent, event.GetType())
	require.Equal(t, events.ValidateMFAAuthResponseFailureCode, event.GetCode())
	require.Equal(t, "test-cluster", event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
	require.False(t, e.Success)
	require.Contains(t, e.Error, "MOCKED TEST ERROR FROM STORAGE LAYER")
}
