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

const (
	chalName      = "test-challenge"
	sourceCluster = "test-cluster"
	targetCluster = "test-cluster"
)

var payload = &mfav1.SessionIdentifyingPayload{
	Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
		SshSessionId: []byte("test-session-id"),
	},
}

func TestCreateValidateSessionChallenge_Webauthn(t *testing.T) {
	t.Parallel()

	authServer, service, emitter, user := setupAuthServer(t, nil)

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
			Payload: payload,
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
	require.Equal(t, sourceCluster, event.GetClusterName())
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
	require.Equal(t, sourceCluster, event.GetClusterName())
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
		targetCluster,
		challengeResp.GetMfaChallenge().GetName(),
	)
	require.NoError(t, err)

	wantedChallenge := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: challengeResp.GetMfaChallenge().GetName(),
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		},
	}

	require.Empty(
		t,
		cmp.Diff(
			wantedChallenge,
			gotChallenge,
			cmp.FilterPath(
				// Ignore expiration time in comparison.
				func(p cmp.Path) bool {
					return p.String() == "Metadata.Expires"
				},
				cmp.Ignore(),
			),
		),
		"GetValidatedMFAChallenge(%s, %s) mismatch (-want +got)", targetCluster, challengeResp.GetMfaChallenge().GetName(),
	)
}

func TestCreateValidateSessionChallenge_SSO(t *testing.T) {
	t.Parallel()

	authServer, service, emitter, user := setupAuthServer(
		t,
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

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	challengeResp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload:              payload,
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
	require.Equal(t, sourceCluster, event.GetClusterName())
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
	require.Equal(t, sourceCluster, event.GetClusterName())
	validateEvent, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), validateEvent.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, validateEvent.FlowType)

	// Verify stored ValidatedMFAChallenge.
	gotChallenge, err := authServer.Auth().MFAService.GetValidatedMFAChallenge(
		t.Context(),
		targetCluster,
		challengeResp.GetMfaChallenge().GetName(),
	)
	require.NoError(t, err)

	wantedChallenge := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: challengeResp.GetMfaChallenge().GetName(),
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
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
	require.Empty(t, diff, "GetValidatedMFAChallenge(%s, %s) mismatch (-want +got):\n%s", targetCluster, challengeResp.GetMfaChallenge().GetName(), diff)
}

func TestCreateSessionChallenge_NonLocalUserDenied(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := t.Context()

	// Use a context with a system user.
	ctx = authz.ContextWithUser(ctx, authtest.TestBuiltin(types.RoleProxy).I)

	_, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: payload,
		},
	)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "only local or remote users can create MFA session challenges")
}

func TestCreateSessionChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	for _, testCase := range []struct {
		name          string
		req           *mfav1.CreateSessionChallengeRequest
		expectedError error
	}{
		{
			name:          "missing payload",
			req:           &mfav1.CreateSessionChallengeRequest{Payload: nil},
			expectedError: trace.NotImplemented("missing or unsupported SessionIdentifyingPayload in request"),
		},
		{
			name: "missing SshSessionId in payload",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte{},
					},
				},
			},
			expectedError: trace.BadParameter("empty SshSessionId in payload"),
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
			expectedError: trace.BadParameter("empty SshSessionId in payload"),
		},
		{
			name: "SSO challenge missing SsoClientRedirectUrl",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload:              payload,
				SsoClientRedirectUrl: "", // missing
				ProxyAddressForSso:   "proxy.example.com",
			},
			expectedError: trace.BadParameter("missing SsoClientRedirectUrl for SSO challenge"),
		},
		{
			name: "SSO challenge missing ProxyAddressForSso",
			req: &mfav1.CreateSessionChallengeRequest{
				Payload:              payload,
				SsoClientRedirectUrl: "https://client/redirect",
				ProxyAddressForSso:   "", // missing
			},
			expectedError: trace.BadParameter("missing ProxyAddressForSso for SSO challenge"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp, err := service.CreateSessionChallenge(ctx, testCase.req)
			require.ErrorIs(t, err, testCase.expectedError)
			require.Nil(t, resp)
		})
	}
}

func TestCreateSessionChallenge_TargetClusterDoesNotExist(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload:       payload,
			TargetCluster: "non-existent-cluster",
		})
	require.True(t, trace.IsNotFound(err))
	require.ErrorContains(t, err, `remote cluster "non-existent-cluster" is not found`)
	require.Nil(t, resp)
}

func TestCreateSessionChallenge_NoMFADevices(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	_, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: payload,
		},
	)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "has no registered MFA devices")
}

func TestValidateSessionChallenge_NonLocalUserDenied(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	// Use a context with a non-local user (e.g., a builtin role).
	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	_, err := service.ValidateSessionChallenge(
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

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	for _, testCase := range []struct {
		name          string
		req           *mfav1.ValidateSessionChallengeRequest
		expectedError string
	}{
		{
			name: "missing MfaResponse",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: nil,
			},
			expectedError: "nil ValidateSessionChallengeRequest.mfa_response",
		},
		{
			name: "missing Response",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: &mfav1.AuthenticateResponse{
					Name:     chalName,
					Response: nil,
				},
			},
			expectedError: "nil ValidateSessionChallengeRequest.mfa_response",
		},
		{
			name: "missing Name",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: &mfav1.AuthenticateResponse{
					Name: "",
					Response: &mfav1.AuthenticateResponse_Webauthn{
						Webauthn: &webauthnpb.CredentialAssertionResponse{}, // minimal, not relevant for this test
					},
				},
			},
			expectedError: "missing ValidateSessionChallengeRequest.mfa_response.name",
		},
		{
			name: "missing Webauthn response",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: &mfav1.AuthenticateResponse{
					Name: chalName,
					Response: &mfav1.AuthenticateResponse_Webauthn{
						Webauthn: nil,
					},
				},
			},
			expectedError: "nil WebauthnResponse in AuthenticateResponse",
		},
		{
			name: "missing SSO response",
			req: &mfav1.ValidateSessionChallengeRequest{
				MfaResponse: &mfav1.AuthenticateResponse{
					Name: chalName,
					Response: &mfav1.AuthenticateResponse_Sso{
						Sso: nil,
					},
				},
			},
			expectedError: "nil SSOResponse in AuthenticateResponse",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp, err := service.ValidateSessionChallenge(ctx, testCase.req)
			require.Nil(t, resp)
			require.True(t, trace.IsBadParameter(err))
			require.ErrorContains(t, err, testCase.expectedError)
		})
	}
}

func TestValidateSessionChallenge_WebauthnFailedValidation(t *testing.T) {
	t.Parallel()

	authServer, service, emitter, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	// Register a Webauthn device for the user.
	_, err := authtest.RegisterTestDevice(
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
			Payload: payload,
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
	require.Equal(t, sourceCluster, event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
	require.False(t, e.Success)
}

func TestValidateSessionChallenge_SSOFailedValidation(t *testing.T) {
	t.Parallel()

	_, service, emitter, user := setupAuthServer(
		t,
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

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	challengeResp, err := service.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload:              payload,
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
	require.Equal(t, sourceCluster, event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
	require.False(t, e.Success)
}

func TestValidateSessionChallenge_WebauthnFailedStorage(t *testing.T) {
	t.Parallel()

	authServer, _, emitter, user := setupAuthServer(t, nil)

	mfaService := &mockMFAService{createValidatedMFAChallengeError: errors.New("MOCKED TEST ERROR FROM STORAGE LAYER")}

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    mfaService,
	})
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
			Payload: payload,
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
	require.Equal(t, sourceCluster, event.GetClusterName())
	e, ok := event.(*apievents.ValidateMFAAuthResponse)
	require.True(t, ok)
	require.Equal(t, user.GetName(), e.GetUser())
	require.Equal(t, apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND, e.FlowType)
	require.False(t, e.Success)
	require.Contains(t, e.Error, "MOCKED TEST ERROR FROM STORAGE LAYER")
}

func TestListValidatedMFAChallenges_Success(t *testing.T) {
	t.Parallel()

	authServer, _, _, user := setupAuthServer(t, nil)

	challenges := []*mfav1.ValidatedMFAChallenge{
		{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: &types.Metadata{
				Name: "test-challenge-1",
			},
			Spec: &mfav1.ValidatedMFAChallengeSpec{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			},
		},
		{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: &types.Metadata{
				Name: "test-challenge-2",
			},
			Spec: &mfav1.ValidatedMFAChallengeSpec{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			},
		},
		{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: &types.Metadata{
				Name: "test-challenge-3",
			},
			Spec: &mfav1.ValidatedMFAChallengeSpec{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			},
		},
	}

	mfaService := &mockMFAService{
		listValidatedMFAChallenges: challenges,
	}
	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    mfaService,
	})
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	resp, err := service.ListValidatedMFAChallenges(
		ctx,
		&mfav1.ListValidatedMFAChallengesRequest{
			PageSize: 3,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	wantResp := &mfav1.ListValidatedMFAChallengesResponse{
		ValidatedChallenges: challenges,
	}

	require.Empty(
		t,
		cmp.Diff(
			wantResp,
			resp,
		), "ListValidatedMFAChallenges mismatch (-want +got)")
}

func TestListValidatedMFAChallenges_NonLocalProxyDenied(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	// Use a context with a non-server role.
	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.ListValidatedMFAChallenges(ctx, &mfav1.ListValidatedMFAChallengesRequest{
		PageSize: 1,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied("only local proxy identities can list validated MFA challenges"))
	require.Nil(t, resp)
}

func TestListValidatedMFAChallenges_InvalidRequest(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	for _, tc := range []struct {
		name          string
		req           *mfav1.ListValidatedMFAChallengesRequest
		expectedError error
	}{
		{
			name: "zero page_size",
			req: &mfav1.ListValidatedMFAChallengesRequest{
				PageSize: 0,
			},
			expectedError: trace.BadParameter("param ListValidatedMFAChallengesRequest.page_size must be a positive integer"),
		},
		{
			name: "negative page_size",
			req: &mfav1.ListValidatedMFAChallengesRequest{
				PageSize: -9000,
			},
			expectedError: trace.BadParameter("param ListValidatedMFAChallengesRequest.page_size must be a positive integer"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.ListValidatedMFAChallenges(ctx, tc.req)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.expectedError)
			require.Nil(t, resp)
		})
	}
}

func TestListValidatedMFAChallenges_FilterByTargetCluster(t *testing.T) {
	t.Parallel()

	authServer, _, _, user := setupAuthServer(t, nil)

	challenges := []*mfav1.ValidatedMFAChallenge{
		{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: &types.Metadata{
				Name: "challenge-for-target-cluster",
			},
			Spec: &mfav1.ValidatedMFAChallengeSpec{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			},
		},
	}

	mfaService := &mockMFAService{
		listValidatedMFAChallenges: challenges,
	}
	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    mfaService,
	})
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	req := &mfav1.ListValidatedMFAChallengesRequest{
		PageSize: 10,
		Filter: &mfav1.ListValidatedMFAChallengesFilter{
			XTargetCluster: &mfav1.ListValidatedMFAChallengesFilter_TargetCluster{
				TargetCluster: targetCluster,
			},
		},
	}

	resp, err := service.ListValidatedMFAChallenges(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.ValidatedChallenges, 1)
	require.Equal(t, "challenge-for-target-cluster", resp.ValidatedChallenges[0].GetMetadata().GetName())
	require.Equal(t, req.GetPageSize(), mfaService.listValidatedMFAChallengesPageSize)
	require.Equal(t, req.GetPageToken(), mfaService.listValidatedMFAChallengesPageToken)
	require.Equal(t, req.GetFilter().GetTargetCluster(), mfaService.listValidatedMFAChallengesTarget)
}

func TestReplicateValidatedMFAChallenge_Success(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, targetCluster).I)

	gotResp, err := service.ReplicateValidatedMFAChallenge(ctx, &mfav1.ReplicateValidatedMFAChallengeRequest{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      user.GetName(),
	})
	require.NoError(t, err)

	wantedResp := &mfav1.ReplicateValidatedMFAChallengeResponse{
		ReplicatedChallenge: &mfav1.ValidatedMFAChallenge{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: &types.Metadata{
				Name: chalName,
			},
			Spec: &mfav1.ValidatedMFAChallengeSpec{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			},
		},
	}
	require.Empty(
		t,
		cmp.Diff(
			wantedResp,
			gotResp,
			cmp.FilterPath(
				// Ignore expiration time in comparison.
				func(p cmp.Path) bool {
					return p.String() == "ReplicatedChallenge.Metadata.Expires"
				},
				cmp.Ignore(),
			),
		),
		"ReplicateValidatedMFAChallenge(%s) mismatch (-want +got)", chalName,
	)
}

func TestReplicateValidatedMFAChallenge_NonRemoteProxyDenied(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	// Use a context with a non-server role.
	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.ReplicateValidatedMFAChallenge(ctx, &mfav1.ReplicateValidatedMFAChallengeRequest{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      "test-user",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied("only remote proxy identities can replicate validated MFA challenges"))
	require.Nil(t, resp)
}

func TestReplicateValidatedMFAChallenge_TargetClusterMismatch(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, targetCluster).I)

	resp, err := service.ReplicateValidatedMFAChallenge(ctx, &mfav1.ReplicateValidatedMFAChallengeRequest{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: "different-cluster",
		Username:      "test-user",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter(`target cluster "different-cluster" does not match current cluster "test-cluster"`))
	require.Nil(t, resp)
}

func TestReplicateValidatedMFAChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, targetCluster).I)

	validReq := &mfav1.ReplicateValidatedMFAChallengeRequest{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      "test-user",
	}

	for _, testCase := range []struct {
		name          string
		req           *mfav1.ReplicateValidatedMFAChallengeRequest
		expectedError error
	}{
		{
			name: "missing Name",
			req: func() *mfav1.ReplicateValidatedMFAChallengeRequest {
				req := *validReq
				req.Name = ""
				return &req
			}(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest name"),
		},
		{
			name: "missing Payload",
			req: func() *mfav1.ReplicateValidatedMFAChallengeRequest {
				req := *validReq
				req.Payload = nil
				return &req
			}(),
			expectedError: trace.NotImplemented("missing or unsupported SessionIdentifyingPayload in request"),
		},
		{
			name: "missing SourceCluster",
			req: func() *mfav1.ReplicateValidatedMFAChallengeRequest {
				req := *validReq
				req.SourceCluster = ""
				return &req
			}(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest source_cluster"),
		},
		{
			name: "missing TargetCluster",
			req: func() *mfav1.ReplicateValidatedMFAChallengeRequest {
				req := *validReq
				req.TargetCluster = ""
				return &req
			}(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest target_cluster"),
		},
		{
			name: "missing Username",
			req: func() *mfav1.ReplicateValidatedMFAChallengeRequest {
				req := *validReq
				req.Username = ""
				return &req
			}(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest username"),
		},
		{
			name: "empty SshSessionId in Payload",
			req: func() *mfav1.ReplicateValidatedMFAChallengeRequest {
				req := *validReq
				req.Payload = &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: []byte{},
					},
				}
				return &req
			}(),
			expectedError: trace.BadParameter("empty SshSessionId in payload"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp, err := service.ReplicateValidatedMFAChallenge(ctx, testCase.req)
			require.ErrorIs(t, err, testCase.expectedError)
			require.Nil(t, resp)
		})
	}
}

func TestVerifyValidatedMFAChallenge_Success(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	chal := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: chalName,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		},
	}
	_, err := authServer.Auth().MFAService.CreateValidatedMFAChallenge(ctx, targetCluster, chal)
	require.NoError(t, err)

	resp, err := service.VerifyValidatedMFAChallenge(
		ctx,
		&mfav1.VerifyValidatedMFAChallengeRequest{
			Username:      user.GetName(),
			Name:          chalName,
			Payload:       payload,
			SourceCluster: sourceCluster,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestVerifyValidatedMFAChallenge_PayloadMismatch(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	chal := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: chalName,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		},
	}
	_, err := authServer.Auth().MFAService.CreateValidatedMFAChallenge(ctx, targetCluster, chal)
	require.NoError(t, err)

	resp, err := service.VerifyValidatedMFAChallenge(ctx, &mfav1.VerifyValidatedMFAChallengeRequest{
		Username: user.GetName(),
		Name:     chalName,
		Payload: &mfav1.SessionIdentifyingPayload{
			Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
				SshSessionId: []byte("this-is-a-different-session-id"),
			},
		},
		SourceCluster: sourceCluster,
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "request payload does not match validated challenge payload")
	require.Nil(t, resp)
}

func TestVerifyValidatedMFAChallenge_SourceClusterMismatch(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	chal := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: chalName,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		},
	}
	_, err := authServer.Auth().MFAService.CreateValidatedMFAChallenge(ctx, targetCluster, chal)
	require.NoError(t, err)

	resp, err := service.VerifyValidatedMFAChallenge(ctx, &mfav1.VerifyValidatedMFAChallengeRequest{
		Username:      user.GetName(),
		Name:          chalName,
		Payload:       payload,
		SourceCluster: "this-is-a-different-cluster",
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "request source cluster does not match validated challenge source cluster")
	require.Nil(t, resp)
}

func TestVerifyValidatedMFAChallenge_NonServerDenied(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	// Use a context with a non-server role.
	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.VerifyValidatedMFAChallenge(ctx, &mfav1.VerifyValidatedMFAChallengeRequest{
		Username:      user.GetName(),
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "only server identities can verify validated MFA challenge")
	require.Nil(t, resp)
}

func TestVerifyValidatedMFAChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	for _, tc := range []struct {
		name          string
		req           *mfav1.VerifyValidatedMFAChallengeRequest
		expectedError error
	}{
		{
			name: "Missing user",
			req: &mfav1.VerifyValidatedMFAChallengeRequest{
				Username:      "",
				Name:          chalName,
				Payload:       payload,
				SourceCluster: sourceCluster,
			},
			expectedError: trace.BadParameter("missing VerifyValidatedMFAChallengeRequest username"),
		},
		{
			name: "Missing name",
			req: &mfav1.VerifyValidatedMFAChallengeRequest{
				Username:      "test-user",
				Name:          "",
				Payload:       payload,
				SourceCluster: sourceCluster,
			},
			expectedError: trace.BadParameter("missing VerifyValidatedMFAChallengeRequest name"),
		},
		{
			name: "Missing payload",
			req: &mfav1.VerifyValidatedMFAChallengeRequest{
				Username:      "test-user",
				Name:          chalName,
				Payload:       nil,
				SourceCluster: sourceCluster,
			},
			expectedError: trace.NotImplemented("missing or unsupported SessionIdentifyingPayload in request"),
		},
		{
			name: "Empty SshSessionId",
			req: &mfav1.VerifyValidatedMFAChallengeRequest{
				Username:      "test-user",
				Name:          chalName,
				Payload:       &mfav1.SessionIdentifyingPayload{Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte{}}},
				SourceCluster: sourceCluster,
			},
			expectedError: trace.BadParameter("empty SshSessionId in payload"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.VerifyValidatedMFAChallenge(ctx, tc.req)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.expectedError)
			require.Nil(t, resp)
		})
	}
}

func TestVerifyValidatedMFAChallenge_NotFound(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	// No challenge stored for this name.
	resp, err := service.VerifyValidatedMFAChallenge(ctx, &mfav1.VerifyValidatedMFAChallengeRequest{
		Username:      user.GetName(),
		Name:          "non-existent-challenge",
		Payload:       payload,
		SourceCluster: sourceCluster,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, resp)
}

func TestVerifyValidatedMFAChallenge_WebauthnFailedStorage(t *testing.T) {
	t.Parallel()

	authServer, _, _, user := setupAuthServer(t, nil)

	mfaService := &mockMFAService{getValidatedMFAChallengeError: errors.New("MOCKED TEST ERROR FROM STORAGE LAYER")}

	service, err := mfav1impl.NewService(mfav1impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().Identity,
		Storage:    mfaService,
	})
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	chal := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: chalName,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		},
	}
	_, err = authServer.Auth().MFAService.CreateValidatedMFAChallenge(ctx, targetCluster, chal)
	require.NoError(t, err)

	resp, err := service.VerifyValidatedMFAChallenge(
		ctx,
		&mfav1.VerifyValidatedMFAChallengeRequest{
			Username:      user.GetName(),
			Name:          chalName,
			Payload:       payload,
			SourceCluster: sourceCluster,
		},
	)
	require.ErrorContains(t, err, "MOCKED TEST ERROR FROM STORAGE LAYER")
	require.Nil(t, resp)
}

func setupAuthServer(t *testing.T, devices []*types.MFADevice) (*mockAuthServer, *mfav1impl.Service, *eventstest.MockRecorderEmitter, types.User) {
	t.Helper()

	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: sourceCluster,
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
		devices,
	)
	require.NoError(t, err)

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
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

	return authServer, service, emitter, user
}
