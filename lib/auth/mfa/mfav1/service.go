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

package mfav1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

// AuthServer defines the subset of lib/auth.Server methods used by the MFA service.
type AuthServer interface {
	BeginSSOMFAChallenge(
		ctx context.Context,
		user string,
		sso *types.SSOMFADevice,
		ssoClientRedirectURL,
		proxyAddress string,
		ext *mfav1.ChallengeExtensions,
		sip *mfav1.SessionIdentifyingPayload,
	) (*proto.SSOChallenge, error)

	VerifySSOMFASession(
		ctx context.Context,
		username,
		sessionID,
		token string,
		ext *mfav1.ChallengeExtensions,
	) (*authz.MFAAuthData, error)
}

// Cache defines the subset of cache methods used by the MFA service.
// See lib/auth.Server.Cache / lib/authclient.Cache.
type Cache interface {
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// Emitter defines the subset of event emitter methods used by the MFA service.
// See lib/auth.Server.Emitter.
type Emitter interface {
	EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error
}

// Identity defines the subset of identity methods used by the MFA service.
// See lib/auth.Server.Identity.
type Identity interface {
	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
	GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error)
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error
	UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error
	GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error)
	DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error
}

// ServiceConfig holds creation parameters for [Service].
type ServiceConfig struct {
	AuthServer AuthServer
	Cache      Cache
	Emitter    Emitter
	Identity   Identity
}

// Service implements the teleport.decision.v1alpha1.DecisionService gRPC API.
type Service struct {
	mfav1.UnimplementedMFAServiceServer

	logger     *slog.Logger
	authServer AuthServer
	cache      Cache
	emitter    Emitter
	identity   Identity
}

// NewService creates a new [Service] instance.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("param AuthServer is required for MFA service")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("param Cache is required for MFA service")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("param Emitter is required for MFA service")
	case cfg.Identity == nil:
		return nil, trace.BadParameter("param Identity is required for MFA service")
	}

	return &Service{
		logger:     slog.With(teleport.ComponentKey, "mfa.service"),
		authServer: cfg.AuthServer,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
		identity:   cfg.Identity,
	}, nil
}

// CreateSessionChallenge implements the mfav1.MFAServiceServer.CreateSessionChallenge method.
func (s *Service) CreateSessionChallenge(
	ctx context.Context,
	req *mfav1.CreateSessionChallengeRequest,
) (*mfav1.CreateSessionChallengeResponse, error) {
	if err := validateCreateSessionChallengeRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	pref, err := s.cache.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Determine which second factors are allowed.
	enableWebauthn, enableSSO := pref.IsSecondFactorWebauthnAllowed(), pref.IsSecondFactorSSOAllowed()

	u2fPref, webConfig, err := mfaPreferences(pref)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devices, err := s.identity.GetMFADevices(ctx, username, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webauthnDevices, ssoDevice := groupByDeviceType(devices)
	if len(webauthnDevices) == 0 && ssoDevice == nil {
		return nil, trace.BadParameter("user %q has no registered MFA devices", username)
	}

	s.logger.DebugContext(
		ctx,
		"Fetched devices for MFA challenge",
		"user", username,
		"num_webauthn_devices", len(webauthnDevices),
		"has_sso_device", ssoDevice != nil,
	)

	challenge := &mfav1.CreateSessionChallengeResponse{MfaChallenge: &mfav1.AuthenticateChallenge{}}

	// If Webauthn is enabled and there are registered devices, create a Webauthn challenge.
	if enableWebauthn && len(webauthnDevices) > 0 {
		webLogin := &wanlib.LoginFlow{
			U2F:      u2fPref,
			Webauthn: webConfig,
			Identity: wanlib.WithDevices(s.identity, webauthnDevices),
		}

		assertion, err := webLogin.Begin(
			ctx,
			username,
			&mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
			req.Payload,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		challenge.MfaChallenge.WebauthnChallenge = wantypes.CredentialAssertionToProto(assertion)
	}

	// If SSO is enabled, the user has an SSO device and the client provided a redirect URL and proxy address, create an
	// SSO challenge.
	if enableSSO && ssoDevice != nil && req.SsoClientRedirectUrl != "" && req.ProxyAddressForSso != "" {
		ssoChallenge, err := s.authServer.BeginSSOMFAChallenge(
			ctx,
			username,
			ssoDevice.GetSso(),
			req.SsoClientRedirectUrl,
			req.ProxyAddressForSso,
			&mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
			req.Payload,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		challenge.MfaChallenge.SsoChallenge = &mfav1.SSOChallenge{
			Device:      ssoDevice.GetSso(),
			RequestId:   ssoChallenge.RequestId,
			RedirectUrl: ssoChallenge.RedirectUrl,
		}
	}

	clusterName, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.CreateMFAAuthChallenge{
		Metadata: apievents.Metadata{
			Type:        events.CreateMFAAuthChallengeEvent,
			Code:        events.CreateMFAAuthChallengeCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
		FlowType:     apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND,
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit CreateMFAAuthChallenge event", "error", err)
	}

	s.logger.DebugContext(
		ctx,
		"Created MFA challenge",
		"user", username,
		"has_webauthn_challenge", challenge.MfaChallenge.WebauthnChallenge != nil,
		"has_sso_challenge", challenge.MfaChallenge.SsoChallenge != nil,
	)

	return challenge, nil
}

// ValidateSessionChallenge implements the mfav1.MFAServiceServer.ValidateSessionChallenge method.
func (s *Service) ValidateSessionChallenge(
	ctx context.Context,
	req *mfav1.ValidateSessionChallengeRequest,
) (*mfav1.ValidateSessionChallengeResponse, error) {
	if err := validateValidateSessionChallengeRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var device *types.MFADevice

	// Validate the challenge response.
	switch resp := req.MfaResponse.GetResponse().(type) {
	case *mfav1.AuthenticateResponse_Webauthn:
		device, err = s.validateWebauthnResponse(ctx, username, resp)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case *mfav1.AuthenticateResponse_Sso:
		device, err = s.validateSSOResponse(ctx, username, resp)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("unknown MFA response type %T", resp)
	}

	// TODO(cthach): Store ValidatedMFAChallenge for retrieval during session creation.

	clusterName, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.ValidateMFAAuthResponse{
		Metadata: apievents.Metadata{
			Type:        events.ValidateMFAAuthResponseEvent,
			Code:        events.ValidateMFAAuthResponseCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
		FlowType:     apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND,
		MFADevice: &apievents.MFADeviceMetadata{
			DeviceName: device.GetName(),
			DeviceID:   device.Id,
			DeviceType: device.MFAType(),
		},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit ValidateMFAAuthResponse event", "error", err)
	}

	s.logger.DebugContext(
		ctx,
		"Validated MFA challenge",
		"user", username,
		"device", device.GetName(),
		"device_type", device.MFAType(),
	)

	return &mfav1.ValidateSessionChallengeResponse{}, nil
}

func validateCreateSessionChallengeRequest(req *mfav1.CreateSessionChallengeRequest) error {
	if req == nil {
		return trace.BadParameter("param CreateSessionChallengeRequest is nil")
	}

	payload := req.GetPayload()
	if payload == nil {
		return trace.BadParameter("missing CreateSessionChallengeRequest payload")
	}

	if len(payload.GetSshSessionId()) == 0 {
		return trace.BadParameter("empty SshSessionId in payload")
	}

	// If either SSO challenge field is set, both must be set.
	if req.SsoClientRedirectUrl != "" || req.ProxyAddressForSso != "" {
		if req.SsoClientRedirectUrl == "" {
			return trace.BadParameter("missing SsoClientRedirectUrl for SSO challenge")
		}

		if req.ProxyAddressForSso == "" {
			return trace.BadParameter("missing ProxyAddressForSso for SSO challenge")
		}
	}

	return nil
}

func groupByDeviceType(devices []*types.MFADevice) ([]*types.MFADevice, *types.MFADevice) {
	var (
		webauthnDevices []*types.MFADevice
		ssoDevice       *types.MFADevice
	)

	// Skip unsupported device types. For example, TOTP devices are not supported for session-based MFA challenges and
	// should be ignored.
	for _, dev := range devices {
		switch dev.Device.(type) {
		case *types.MFADevice_U2F, *types.MFADevice_Webauthn:
			webauthnDevices = append(webauthnDevices, dev)
		case *types.MFADevice_Sso:
			ssoDevice = dev
		}
	}

	return webauthnDevices, ssoDevice
}

func validateValidateSessionChallengeRequest(req *mfav1.ValidateSessionChallengeRequest) error {
	if req == nil {
		return trace.BadParameter("param ValidateSessionChallengeRequest is nil")
	}

	mfaResp := req.GetMfaResponse()
	if mfaResp == nil {
		return trace.BadParameter("nil ValidateSessionChallengeRequest.mfa_response")
	}

	resp := mfaResp.GetResponse()
	if resp == nil {
		return trace.BadParameter("nil ValidateSessionChallengeRequest.mfa_response.response")
	}

	if mfaResp.GetWebauthn() == nil && mfaResp.GetSso() == nil {
		return trace.BadParameter("one of WebauthnResponse or SSOResponse must be provided")
	}

	return nil
}

func (s *Service) validateWebauthnResponse(
	ctx context.Context,
	username string,
	resp *mfav1.AuthenticateResponse_Webauthn,
) (*types.MFADevice, error) {
	pref, err := s.cache.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u2fPref, webConfig, err := mfaPreferences(pref)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webLogin := &wanlib.LoginFlow{
		U2F:      u2fPref,
		Webauthn: webConfig,
		Identity: s.identity,
	}

	loginData, err := webLogin.Finish(
		ctx,
		username,
		wantypes.CredentialAssertionResponseFromProto(resp.Webauthn),
		&mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
	)
	if err != nil {
		return nil, trace.AccessDenied("validate Webauthn response: %v", err)
	}

	return loginData.Device, nil
}

func (s *Service) validateSSOResponse(
	ctx context.Context,
	username string,
	resp *mfav1.AuthenticateResponse_Sso,
) (*types.MFADevice, error) {
	data, err := s.authServer.VerifySSOMFASession(
		ctx,
		username,
		resp.Sso.RequestId,
		resp.Sso.Token,
		&mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
	)
	if err != nil {
		return nil, trace.AccessDenied("validate SSO response: %v", err)
	}

	return data.Device, nil
}

func mfaPreferences(pref types.AuthPreference) (*types.U2F, *types.Webauthn, error) {
	// Get the user's U2F preference. If it doesn't exist, continue since U2F may not be enabled.
	u2f, err := pref.GetU2F()
	if err != nil && !trace.IsNotFound(err) {
		return nil, nil, trace.Wrap(err)
	}

	// Get the user's WebAuthn preference. If it doesn't exist, continue since WebAuthn may not be enabled.
	webauthn, err := pref.GetWebauthn()
	if err != nil && !trace.IsNotFound(err) {
		return nil, nil, trace.Wrap(err)
	}

	return u2f, webauthn, nil
}
