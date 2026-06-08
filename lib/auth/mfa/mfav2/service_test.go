// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package mfav2_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authtest"
	mfav2impl "github.com/gravitational/teleport/lib/auth/mfa/mfav2"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
)

const (
	chalName      = "test-challenge"
	sourceCluster = "test-cluster"
	targetCluster = "test-cluster"
)

var payload = mfav2.SessionIdentifyingPayload_builder{
	SshSessionId: []byte("test-session-id"),
}.Build()

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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload: payload,
		}.Build(),
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
		WebauthnChallenge: webauthnpb.CredentialAssertionV2ToV1(challengeResp.GetMfaChallenge().GetWebauthnChallenge()),
	}

	mfaResp, err := device.SolveAuthn(challenge)
	require.NoError(t, err)

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfav2.AuthenticateResponse_builder{
				Name:     challengeResp.GetMfaChallenge().GetName(),
				Webauthn: webauthnpb.CredentialAssertionResponseV1ToV2(mfaResp.GetWebauthn()),
			}.Build(),
		}.Build(),
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

	wantedChallenge := mfav2.ValidatedMFAChallenge_builder{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: challengeResp.GetMfaChallenge().GetName(),
		}.Build(),
		Spec: mfav2.ValidatedMFAChallengeSpec_builder{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		}.Build(),
	}.Build()

	require.Empty(
		t,
		cmp.Diff(
			wantedChallenge,
			gotChallenge,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload:              payload,
			SsoClientRedirectUrl: "https://sso/redirect",
			ProxyAddressForSso:   "proxy.example.com",
		}.Build(),
	)
	require.NoError(t, err)

	// Verify SSO challenge was returned.
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetSsoChallenge(), "SSOChallenge must not be nil")
	require.Equal(t, "test-display-name", challengeResp.GetMfaChallenge().GetSsoChallenge().GetDevice().GetDisplayName())
	require.Equal(t, "test-device-connector-id", challengeResp.GetMfaChallenge().GetSsoChallenge().GetDevice().GetConnectorId())
	require.Equal(t, constants.SAML, challengeResp.GetMfaChallenge().GetSsoChallenge().GetDevice().GetConnectorType())
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
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfav2.AuthenticateResponse_builder{
				Name: challengeResp.GetMfaChallenge().GetName(),
				Sso: mfav2.SSOChallengeResponse_builder{
					RequestId: challengeResp.GetMfaChallenge().GetSsoChallenge().GetRequestId(),
				}.Build(),
			}.Build(),
		}.Build(),
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

	wantedChallenge := mfav2.ValidatedMFAChallenge_builder{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: challengeResp.GetMfaChallenge().GetName(),
		}.Build(),
		Spec: mfav2.ValidatedMFAChallengeSpec_builder{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		}.Build(),
	}.Build()

	diff := cmp.Diff(
		wantedChallenge,
		gotChallenge,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload: payload,
		}.Build(),
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
		req           *mfav2.CreateSessionChallengeRequest
		expectedError error
	}{
		{
			name:          "missing payload",
			req:           mfav2.CreateSessionChallengeRequest_builder{Payload: nil}.Build(),
			expectedError: trace.BadParameter("missing SessionIdentifyingPayload in request"),
		},
		{
			name: "missing SshSessionId in payload",
			req: mfav2.CreateSessionChallengeRequest_builder{
				Payload: mfav2.SessionIdentifyingPayload_builder{
					SshSessionId: []byte{},
				}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("empty SshSessionId in payload"),
		},
		{
			name: "empty SshSessionId in payload",
			req: mfav2.CreateSessionChallengeRequest_builder{
				Payload: mfav2.SessionIdentifyingPayload_builder{
					SshSessionId: []byte{},
				}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("empty SshSessionId in payload"),
		},
		{
			name: "SSO challenge missing SsoClientRedirectUrl",
			req: mfav2.CreateSessionChallengeRequest_builder{
				Payload:              payload,
				SsoClientRedirectUrl: "", // missing
				ProxyAddressForSso:   "proxy.example.com",
			}.Build(),
			expectedError: trace.BadParameter("missing SsoClientRedirectUrl for SSO challenge"),
		},
		{
			name: "SSO challenge missing ProxyAddressForSso",
			req: mfav2.CreateSessionChallengeRequest_builder{
				Payload:              payload,
				SsoClientRedirectUrl: "https://client/redirect",
				ProxyAddressForSso:   "", // missing
			}.Build(),
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload:       payload,
			TargetCluster: "non-existent-cluster",
		}.Build(),
	)
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload: payload,
		}.Build(),
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
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfav2.AuthenticateResponse_builder{
				Webauthn: nil, // minimal, not relevant for this test
			}.Build(),
		}.Build(),
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
		req           *mfav2.ValidateSessionChallengeRequest
		expectedError string
	}{
		{
			name: "missing MfaResponse",
			req: mfav2.ValidateSessionChallengeRequest_builder{
				MfaResponse: nil,
			}.Build(),
			expectedError: "nil ValidateSessionChallengeRequest.mfa_response",
		},
		{
			name: "missing Response",
			req: mfav2.ValidateSessionChallengeRequest_builder{
				MfaResponse: mfav2.AuthenticateResponse_builder{
					Name: chalName,
				}.Build(),
			}.Build(),
			expectedError: "missing Webauthn or SSO response in AuthenticateResponse",
		},
		{
			name: "missing Name",
			req: mfav2.ValidateSessionChallengeRequest_builder{
				MfaResponse: mfav2.AuthenticateResponse_builder{
					Name:     "",
					Webauthn: webauthnpb.CredentialAssertionResponseV1ToV2(&webauthnpb.CredentialAssertionResponse{}), // minimal, not relevant for this test
				}.Build(),
			}.Build(),
			expectedError: "missing ValidateSessionChallengeRequest.mfa_response.name",
		},
		{
			name: "missing Webauthn response",
			req: mfav2.ValidateSessionChallengeRequest_builder{
				MfaResponse: mfav2.AuthenticateResponse_builder{
					Name:     chalName,
					Webauthn: nil,
				}.Build(),
			}.Build(),
			expectedError: "missing Webauthn or SSO response in AuthenticateResponse",
		},
		{
			name: "missing SSO response",
			req: mfav2.ValidateSessionChallengeRequest_builder{
				MfaResponse: mfav2.AuthenticateResponse_builder{
					Name: chalName,
					Sso:  nil,
				}.Build(),
			}.Build(),
			expectedError: "missing Webauthn or SSO response in AuthenticateResponse",
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload: payload,
		}.Build(),
	)
	require.NoError(t, err)
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetWebauthnChallenge(), "WebauthnChallenge must not be nil")

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfav2.AuthenticateResponse_builder{
				Name: challengeResp.GetMfaChallenge().GetName(),
				Webauthn: webauthnpb.CredentialAssertionResponseV1ToV2(&webauthnpb.CredentialAssertionResponse{
					Type: "invalid",
				}),
			}.Build(),
		}.Build(),
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload:              payload,
			SsoClientRedirectUrl: "https://sso/redirect",
			ProxyAddressForSso:   "proxy.example.com",
		}.Build(),
	)
	require.NoError(t, err)

	// Verify SSO challenge was returned.
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetSsoChallenge(), "SSOChallenge must not be nil")

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfav2.AuthenticateResponse_builder{
				Name: challengeResp.GetMfaChallenge().GetName(),
				Sso: mfav2.SSOChallengeResponse_builder{
					RequestId: "invalid-request-id-to-fail-validation",
				}.Build(),
			}.Build(),
		}.Build(),
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

	service, err := mfav2impl.NewService(mfav2impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().IdentityInternal,
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
		mfav2.CreateSessionChallengeRequest_builder{
			Payload: payload,
		}.Build(),
	)
	require.NoError(t, err)
	require.NotEmpty(t, challengeResp.GetMfaChallenge().GetName(), "Challenge name must not be empty")
	require.NotNil(t, challengeResp.GetMfaChallenge().GetWebauthnChallenge(), "WebauthnChallenge must not be nil")

	challenge := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: webauthnpb.CredentialAssertionV2ToV1(challengeResp.GetMfaChallenge().GetWebauthnChallenge()),
	}

	mfaResp, err := device.SolveAuthn(challenge)
	require.NoError(t, err)

	validateResp, err := service.ValidateSessionChallenge(
		ctx,
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfav2.AuthenticateResponse_builder{
				Name:     challengeResp.GetMfaChallenge().GetName(),
				Webauthn: webauthnpb.CredentialAssertionResponseV1ToV2(mfaResp.GetWebauthn()),
			}.Build(),
		}.Build(),
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

	challenges := []*mfav2.ValidatedMFAChallenge{
		mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "test-challenge-1",
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			}.Build(),
		}.Build(),
		mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "test-challenge-2",
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			}.Build(),
		}.Build(),
		mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "test-challenge-3",
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			}.Build(),
		}.Build(),
	}

	mfaService := &mockMFAService{
		listValidatedMFAChallenges: challenges,
	}
	service, err := mfav2impl.NewService(mfav2impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().IdentityInternal,
		Storage:    mfaService,
	})
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	resp, err := service.ListValidatedMFAChallenges(
		ctx,
		mfav2.ListValidatedMFAChallengesRequest_builder{
			PageSize: 3,
		}.Build(),
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	wantResp := mfav2.ListValidatedMFAChallengesResponse_builder{
		ValidatedChallenges: challenges,
	}.Build()

	require.Empty(
		t,
		cmp.Diff(
			wantResp,
			resp,
			protocmp.Transform(),
		), "ListValidatedMFAChallenges mismatch (-want +got)")
}

func TestListValidatedMFAChallenges_NonLocalProxyDenied(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	// Use a context with a non-server role.
	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.ListValidatedMFAChallenges(ctx, mfav2.ListValidatedMFAChallengesRequest_builder{
		PageSize: 1,
	}.Build())
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
		req           *mfav2.ListValidatedMFAChallengesRequest
		expectedError error
	}{
		{
			name: "zero page_size",
			req: mfav2.ListValidatedMFAChallengesRequest_builder{
				PageSize: 0,
			}.Build(),
			expectedError: trace.BadParameter("param ListValidatedMFAChallengesRequest.page_size must be a positive integer"),
		},
		{
			name: "negative page_size",
			req: mfav2.ListValidatedMFAChallengesRequest_builder{
				PageSize: -9000,
			}.Build(),
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

	challenges := []*mfav2.ValidatedMFAChallenge{
		mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "challenge-for-target-cluster",
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			}.Build(),
		}.Build(),
	}

	mfaService := &mockMFAService{
		listValidatedMFAChallenges: challenges,
	}
	service, err := mfav2impl.NewService(mfav2impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().IdentityInternal,
		Storage:    mfaService,
	})
	require.NoError(t, err)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	tc := targetCluster
	req := mfav2.ListValidatedMFAChallengesRequest_builder{
		PageSize: 10,
		Filter: mfav2.ListValidatedMFAChallengesFilter_builder{
			TargetCluster: &tc,
		}.Build(),
	}.Build()

	resp, err := service.ListValidatedMFAChallenges(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.GetValidatedChallenges(), 1)
	require.Equal(t, "challenge-for-target-cluster", resp.GetValidatedChallenges()[0].GetMetadata().GetName())
	require.Equal(t, req.GetPageSize(), mfaService.listValidatedMFAChallengesPageSize)
	require.Equal(t, req.GetPageToken(), mfaService.listValidatedMFAChallengesPageToken)
	require.Equal(t, req.GetFilter().GetTargetCluster(), mfaService.listValidatedMFAChallengesTarget)
}

func TestReplicateValidatedMFAChallenge_Success(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, targetCluster).I)

	gotResp, err := service.ReplicateValidatedMFAChallenge(ctx, mfav2.ReplicateValidatedMFAChallengeRequest_builder{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      user.GetName(),
	}.Build())
	require.NoError(t, err)

	wantedResp := mfav2.ReplicateValidatedMFAChallengeResponse_builder{
		ReplicatedChallenge: mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: chalName,
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			}.Build(),
		}.Build(),
	}.Build()
	require.Empty(
		t,
		cmp.Diff(
			wantedResp,
			gotResp,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
		),
		"ReplicateValidatedMFAChallenge(%s) mismatch (-want +got)", chalName,
	)
}

func TestReplicateValidatedMFAChallenge_NonRemoteProxyDenied(t *testing.T) {
	t.Parallel()

	_, service, _, user := setupAuthServer(t, nil)

	// Use a context with a non-server role.
	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.ReplicateValidatedMFAChallenge(ctx, mfav2.ReplicateValidatedMFAChallengeRequest_builder{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      "test-user",
	}.Build())
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied("identity is not a remote builtin role, cannot be a remote proxy"))
	require.Nil(t, resp)
}

func TestReplicateValidatedMFAChallenge_RemoteBuiltinWrongRoleDenied(t *testing.T) {
	t.Parallel()

	authServer, _, _, _ := setupAuthServer(t, nil)

	service, err := mfav2impl.NewService(mfav2impl.ServiceConfig{
		Authorizer: authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
			identity := authz.RemoteBuiltinRole{
				Role:        types.RoleNode,
				Username:    string(types.RoleNode),
				ClusterName: sourceCluster,
			}

			return &authz.Context{
				Identity:         identity,
				UnmappedIdentity: identity,
				Checker:          mockAccessChecker{},
			}, nil
		}),
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().IdentityInternal,
		Storage:    &mockMFAService{},
	})
	require.NoError(t, err)

	resp, err := service.ReplicateValidatedMFAChallenge(t.Context(), mfav2.ReplicateValidatedMFAChallengeRequest_builder{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      "test-user",
	}.Build())
	require.Error(t, err)
	require.ErrorIs(
		t,
		err,
		trace.AccessDenied("role %q does not have permission to replicate validated MFA challenges", types.RoleNode),
	)
	require.Nil(t, resp)
}

func TestReplicateValidatedMFAChallenge_RemoteProxyWrongClusterDenied(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	// Use a context with a remote proxy identity, but for a different source cluster than the one in the request.
	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, "different-source-cluster").I)

	resp, err := service.ReplicateValidatedMFAChallenge(ctx, mfav2.ReplicateValidatedMFAChallengeRequest_builder{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: targetCluster,
		Username:      "test-user",
	}.Build())
	require.Error(t, err)
	require.ErrorIs(
		t,
		err,
		trace.AccessDenied(
			"remote proxy cluster %q does not match request source cluster %q",
			"different-source-cluster",
			sourceCluster,
		),
	)
	require.Nil(t, resp)
}

func TestReplicateValidatedMFAChallenge_TargetClusterMismatch(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, targetCluster).I)

	resp, err := service.ReplicateValidatedMFAChallenge(ctx, mfav2.ReplicateValidatedMFAChallengeRequest_builder{
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
		TargetCluster: "different-cluster",
		Username:      "test-user",
	}.Build())
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter(`target cluster "different-cluster" does not match current cluster "test-cluster"`))
	require.Nil(t, resp)
}

func TestReplicateValidatedMFAChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestRemoteBuiltin(types.RoleProxy, targetCluster).I)

	for _, testCase := range []struct {
		name          string
		req           *mfav2.ReplicateValidatedMFAChallengeRequest
		expectedError error
	}{
		{
			name: "missing Name",
			req: mfav2.ReplicateValidatedMFAChallengeRequest_builder{
				Name:          "",
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      "test-user",
			}.Build(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest name"),
		},
		{
			name: "missing Payload",
			req: mfav2.ReplicateValidatedMFAChallengeRequest_builder{
				Name:          chalName,
				Payload:       nil,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      "test-user",
			}.Build(),
			expectedError: trace.BadParameter("missing SessionIdentifyingPayload in request"),
		},
		{
			name: "missing SourceCluster",
			req: mfav2.ReplicateValidatedMFAChallengeRequest_builder{
				Name:          chalName,
				Payload:       payload,
				SourceCluster: "",
				TargetCluster: targetCluster,
				Username:      "test-user",
			}.Build(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest source_cluster"),
		},
		{
			name: "missing TargetCluster",
			req: mfav2.ReplicateValidatedMFAChallengeRequest_builder{
				Name:          chalName,
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: "",
				Username:      "test-user",
			}.Build(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest target_cluster"),
		},
		{
			name: "missing Username",
			req: mfav2.ReplicateValidatedMFAChallengeRequest_builder{
				Name:          chalName,
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      "",
			}.Build(),
			expectedError: trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest username"),
		},
		{
			name: "empty SshSessionId in Payload",
			req: mfav2.ReplicateValidatedMFAChallengeRequest_builder{
				Name: chalName,
				Payload: mfav2.SessionIdentifyingPayload_builder{
					SshSessionId: []byte{},
				}.Build(),
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      "test-user",
			}.Build(),
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

	ctx, cancel := context.WithTimeout(
		authz.ContextWithUser(
			t.Context(),
			authtest.TestBuiltin(types.RoleNode).I,
		),
		5*time.Second,
	)
	defer cancel()

	req := mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Username:      user.GetName(),
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
	}.Build()

	group, ctx := errgroup.WithContext(ctx)

	// Start a goroutine to create the ValidatedMFAChallenge, to simulate the expected real-world sequence of events
	// where the challenge is created before it is verified, but not necessarily immediately before.
	group.Go(func() error {
		chal := mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: chalName,
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload:       payload,
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      user.GetName(),
			}.Build(),
		}.Build()

		if _, err := authServer.Auth().CreateValidatedMFAChallenge(ctx, targetCluster, chal); err != nil {
			return trace.Wrap(err, "create ValidatedMFAChallenge")
		}

		return nil
	})

	// Start a goroutine to verify the ValidatedMFAChallenge, which will wait until the challenge is created by the
	// first goroutine.
	group.Go(func() error {
		resp, err := service.VerifyValidatedMFAChallenge(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		if resp == nil {
			return trace.BadParameter("expected non-nil response")
		}

		return nil
	})

	// Wait for both goroutines to complete and check for errors. The fact that the verify goroutine does not return an
	// error indicates that the challenge was successfully verified asynchronously after it was created.
	require.NoError(t, group.Wait())
}

func TestVerifyValidatedMFAChallenge_PayloadMismatch(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	chal := mfav2.ValidatedMFAChallenge_builder{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: chalName,
		}.Build(),
		Spec: mfav2.ValidatedMFAChallengeSpec_builder{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		}.Build(),
	}.Build()
	_, err := authServer.Auth().MFAService.CreateValidatedMFAChallenge(ctx, targetCluster, chal)
	require.NoError(t, err)

	resp, err := service.VerifyValidatedMFAChallenge(ctx, mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Username: user.GetName(),
		Name:     chalName,
		Payload: mfav2.SessionIdentifyingPayload_builder{
			SshSessionId: []byte("this-is-a-different-session-id"),
		}.Build(),
		SourceCluster: sourceCluster,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "request payload does not match validated challenge payload")
	require.Nil(t, resp)
}

func TestVerifyValidatedMFAChallenge_SourceClusterMismatch(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I)

	chal := mfav2.ValidatedMFAChallenge_builder{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: chalName,
		}.Build(),
		Spec: mfav2.ValidatedMFAChallengeSpec_builder{
			Payload:       payload,
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
			Username:      user.GetName(),
		}.Build(),
	}.Build()
	_, err := authServer.Auth().MFAService.CreateValidatedMFAChallenge(ctx, targetCluster, chal)
	require.NoError(t, err)

	resp, err := service.VerifyValidatedMFAChallenge(ctx, mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Username:      user.GetName(),
		Name:          chalName,
		Payload:       payload,
		SourceCluster: "this-is-a-different-cluster",
	}.Build())
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

	resp, err := service.VerifyValidatedMFAChallenge(ctx, mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Username:      user.GetName(),
		Name:          chalName,
		Payload:       payload,
		SourceCluster: sourceCluster,
	}.Build())
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
		req           *mfav2.VerifyValidatedMFAChallengeRequest
		expectedError error
	}{
		{
			name: "Missing user",
			req: mfav2.VerifyValidatedMFAChallengeRequest_builder{
				Username:      "",
				Name:          chalName,
				Payload:       payload,
				SourceCluster: sourceCluster,
			}.Build(),
			expectedError: trace.BadParameter("missing VerifyValidatedMFAChallengeRequest username"),
		},
		{
			name: "Missing name",
			req: mfav2.VerifyValidatedMFAChallengeRequest_builder{
				Username:      "test-user",
				Name:          "",
				Payload:       payload,
				SourceCluster: sourceCluster,
			}.Build(),
			expectedError: trace.BadParameter("missing VerifyValidatedMFAChallengeRequest name"),
		},
		{
			name: "Missing payload",
			req: mfav2.VerifyValidatedMFAChallengeRequest_builder{
				Username:      "test-user",
				Name:          chalName,
				Payload:       nil,
				SourceCluster: sourceCluster,
			}.Build(),
			expectedError: trace.BadParameter("missing SessionIdentifyingPayload in request"),
		},
		{
			name: "Empty SshSessionId",
			req: mfav2.VerifyValidatedMFAChallengeRequest_builder{
				Username:      "test-user",
				Name:          chalName,
				Payload:       mfav2.SessionIdentifyingPayload_builder{SshSessionId: []byte{}}.Build(),
				SourceCluster: sourceCluster,
			}.Build(),
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

	ctx, cancel := context.WithTimeout(
		authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleNode).I),
		time.Millisecond,
	)
	defer cancel()

	// No challenge stored for this name or the challenge was not created within the context's timeout.
	resp, err := service.VerifyValidatedMFAChallenge(ctx, mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Username:      user.GetName(),
		Name:          "non-existent-challenge",
		Payload:       payload,
		SourceCluster: sourceCluster,
	}.Build())
	require.True(t, trace.IsLimitExceeded(err))
	require.Nil(t, resp)
}

func setupAuthServer(t *testing.T, devices []*types.MFADevice) (*mockAuthServer, *mfav2impl.Service, *eventstest.MockRecorderEmitter, types.User) {
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

	t.Cleanup(func() {
		if err := authServer.Close(); err != nil {
			t.Logf("Failed to close auth server: %v", err)
		}
	})

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	service, err := mfav2impl.NewService(mfav2impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
		Cache:      authServer.Auth().Cache,
		Emitter:    authServer.Auth(),
		Identity:   authServer.Auth().IdentityInternal,
		Storage:    authServer.Auth(),
	})
	require.NoError(t, err)

	return authServer, service, emitter, user
}

type mockAccessChecker struct {
	services.AccessChecker

	roles map[string]bool
}

func (f mockAccessChecker) HasRole(role string) bool {
	return f.roles[role]
}
