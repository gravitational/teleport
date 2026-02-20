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
	"bytes"
	"cmp"
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

// AuthServer defines the subset of lib/auth.Server methods used by the MFA service.
// TODO(cthach): Remove after SSO MFA device support is added to lib/auth/authtest
// (https://github.com/gravitational/teleport/issues/62271) and update the tests to use lib/auth/authtest for mocking.
type AuthServer interface {
	BeginSSOMFAChallenge(
		ctx context.Context,
		params mfatypes.BeginSSOMFAChallengeParams,
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
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
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

// MFAService defines the interface for managing MFA resources.
// See lib/auth.MFAService.
type MFAService interface {
	CreateValidatedMFAChallenge(ctx context.Context,
		targetCluster string,
		chal *mfav1.ValidatedMFAChallenge,
	) (*mfav1.ValidatedMFAChallenge, error)

	GetValidatedMFAChallenge(
		ctx context.Context,
		targetCluster string,
		challengeName string,
	) (*mfav1.ValidatedMFAChallenge, error)

	ListValidatedMFAChallenges(
		ctx context.Context,
		pageSize int32,
		pageToken string,
		targetCluster string,
	) ([]*mfav1.ValidatedMFAChallenge, string, error)
}

// ServiceConfig holds creation parameters for [Service].
type ServiceConfig struct {
	Authorizer authz.Authorizer
	AuthServer AuthServer
	Cache      Cache
	Emitter    Emitter
	Identity   Identity
	Storage    MFAService
}

// Service implements the teleport.decision.v1alpha1.DecisionService gRPC API.
type Service struct {
	mfav1.UnimplementedMFAServiceServer

	logger     *slog.Logger
	authorizer authz.Authorizer
	authServer AuthServer
	cache      Cache
	emitter    Emitter
	identity   Identity
	storage    MFAService
}

// NewService creates a new [Service] instance.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("param Authorizer is required for MFA service")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("param AuthServer is required for MFA service")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("param Cache is required for MFA service")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("param Emitter is required for MFA service")
	case cfg.Identity == nil:
		return nil, trace.BadParameter("param Identity is required for MFA service")
	case cfg.Storage == nil:
		return nil, trace.BadParameter("param Storage is required for MFA service")
	}

	return &Service{
		logger:     slog.With(teleport.ComponentKey, "mfa.service"),
		authorizer: cfg.Authorizer,
		authServer: cfg.AuthServer,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
		identity:   cfg.Identity,
		storage:    cfg.Storage,
	}, nil
}

// CreateSessionChallenge implements the mfav1.MFAServiceServer.CreateSessionChallenge method.
func (s *Service) CreateSessionChallenge(
	ctx context.Context,
	req *mfav1.CreateSessionChallengeRequest,
) (*mfav1.CreateSessionChallengeResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteUser(*authCtx) {
		return nil, trace.AccessDenied("only local or remote users can create MFA session challenges")
	}

	if err := validateCreateSessionChallengeRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	// If a target cluster is specified, ensure that it is a valid cluster that exists.
	if req.TargetCluster != "" {
		if err := s.clusterExists(ctx, req.TargetCluster); err != nil {
			return nil, trace.Wrap(err)
		}
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

	supportedMFADevices := s.groupAndFilterSupportedMFADevices(ctx, username, devices)
	if len(supportedMFADevices.Webauthn) == 0 && supportedMFADevices.SSO == nil {
		return nil, trace.BadParameter("user %q has no registered MFA devices", username)
	}

	s.logger.DebugContext(
		ctx,
		"Fetched devices for challenge",
		"user", username,
		"num_webauthn_devices", len(supportedMFADevices.Webauthn),
		"has_sso_device", supportedMFADevices.SSO != nil,
	)

	// Create the MFA challenge response with a randomly generated UUID for its name. This name is used to track the
	// status of the MFA challenge throughout its lifecycle by the service that the user is authenticating to. The key
	// is scoped by target cluster and the actual challenge has a short TTL, so collisions are extremely unlikely.
	challenge := &mfav1.CreateSessionChallengeResponse{
		MfaChallenge: &mfav1.AuthenticateChallenge{
			Name: uuid.NewString(),
		},
	}

	currentCluster, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Source cluster is always the current cluster.
	sourceClusterName := currentCluster.GetClusterName()

	// Target cluster is the current cluster unless specified otherwise in the request.
	targetClusterName := cmp.Or(req.TargetCluster, sourceClusterName)

	// If Webauthn is enabled and there are registered devices, create a Webauthn challenge.
	if enableWebauthn && len(supportedMFADevices.Webauthn) > 0 {
		webLogin := &wanlib.LoginFlow{
			U2F:      u2fPref,
			Webauthn: webConfig,
			Identity: wanlib.WithDevices(s.identity, supportedMFADevices.Webauthn),
		}

		assertion, err := webLogin.Begin(
			ctx,
			wanlib.BeginParams{
				User:                      username,
				ChallengeExtensions:       &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
				SessionIdentifyingPayload: req.Payload,
				SourceCluster:             sourceClusterName,
				TargetCluster:             targetClusterName,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		challenge.MfaChallenge.WebauthnChallenge = wantypes.CredentialAssertionToProto(assertion)
	}

	// If SSO is enabled, the user has an SSO device and the client provided a redirect URL and proxy address, create an
	// SSO challenge.
	if enableSSO && supportedMFADevices.SSO != nil && req.SsoClientRedirectUrl != "" && req.ProxyAddressForSso != "" {
		ssoChallenge, err := s.authServer.BeginSSOMFAChallenge(
			ctx,
			mfatypes.BeginSSOMFAChallengeParams{
				User:                 username,
				SSO:                  supportedMFADevices.SSO.GetSso(),
				SSOClientRedirectURL: req.SsoClientRedirectUrl,
				ProxyAddress:         req.ProxyAddressForSso,
				Ext:                  &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
				SIP:                  req.Payload,
				SourceCluster:        sourceClusterName,
				TargetCluster:        targetClusterName,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		challenge.MfaChallenge.SsoChallenge = &mfav1.SSOChallenge{
			Device:      supportedMFADevices.SSO.GetSso(),
			RequestId:   ssoChallenge.RequestId,
			RedirectUrl: ssoChallenge.RedirectUrl,
		}
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.CreateMFAAuthChallenge{
		Metadata: apievents.Metadata{
			Type:        events.CreateMFAAuthChallengeEvent,
			Code:        events.CreateMFAAuthChallengeCode,
			ClusterName: currentCluster.GetClusterName(),
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
		FlowType:     apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND,
	}); err != nil {
		s.logger.ErrorContext(ctx, "Failed to emit CreateMFAAuthChallenge event", "error", err)
	}

	s.logger.DebugContext(
		ctx,
		"Created challenge",
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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteUser(*authCtx) {
		return nil, trace.AccessDenied("only local or remote users can validate MFA session challenges")
	}

	if err := validateValidateSessionChallengeRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currentCluster, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var details *authz.MFAAuthData

	// Validate the challenge response.
	switch resp := req.MfaResponse.GetResponse().(type) {
	case *mfav1.AuthenticateResponse_Webauthn:
		details, err = s.validateWebauthnResponse(ctx, username, resp)

	case *mfav1.AuthenticateResponse_Sso:
		details, err = s.validateSSOResponse(ctx, username, resp)

	default:
		return nil, trace.BadParameter("unknown MFA response type %T", resp)
	}

	if err != nil {
		// Emit failure event before returning.
		s.emitValidationEvent(ctx, currentCluster.GetClusterName(), username, nil, err)

		return nil, trace.AccessDenied("validate MFA challenge response: %v", err)
	}

	// Store the validated challenge resource.
	_, err = s.storage.CreateValidatedMFAChallenge(
		ctx,
		details.TargetCluster,
		&mfav1.ValidatedMFAChallenge{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: &types.Metadata{
				Name: req.GetMfaResponse().GetName(),
			},
			Spec: &mfav1.ValidatedMFAChallengeSpec{
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: details.Payload.SSHSessionID,
					},
				},
				SourceCluster: details.SourceCluster,
				TargetCluster: details.TargetCluster,
				Username:      username,
			},
		},
	)
	if err != nil {
		// Emit failure event before returning.
		s.emitValidationEvent(ctx, currentCluster.GetClusterName(), username, nil /* device */, err)

		return nil, trace.AccessDenied("validate MFA challenge response: %v", err)
	}

	// Emit success event.
	s.emitValidationEvent(ctx, currentCluster.GetClusterName(), username, details.Device, nil)

	return &mfav1.ValidateSessionChallengeResponse{}, nil
}

// ListValidatedMFAChallenges implements the mfav1.MFAServiceServer.ListValidatedMFAChallenges method.
func (s *Service) ListValidatedMFAChallenges(
	ctx context.Context,
	req *mfav1.ListValidatedMFAChallengesRequest,
) (*mfav1.ListValidatedMFAChallengesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !isLocalProxy(*authCtx) {
		return nil, trace.AccessDenied("only local proxy identities can list validated MFA challenges")
	}

	if err := checkListValidatedMFAChallengesRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	// If a filter is provided with a target cluster, use it to scope the listing of validated MFA challenges.
	targetCluster := cmp.Or(req.GetFilter().GetTargetCluster(), "")

	challenges, nextPageToken, err := s.storage.ListValidatedMFAChallenges(
		ctx,
		req.GetPageSize(),
		req.GetPageToken(),
		targetCluster,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &mfav1.ListValidatedMFAChallengesResponse{
		ValidatedChallenges: challenges,
		NextPageToken:       nextPageToken,
	}, nil
}

// ReplicateValidatedMFAChallenge implements the mfav1.MFAServiceServer.ReplicateValidatedMFAChallenge method.
func (s *Service) ReplicateValidatedMFAChallenge(
	ctx context.Context,
	req *mfav1.ReplicateValidatedMFAChallengeRequest,
) (*mfav1.ReplicateValidatedMFAChallengeResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !isRemoteProxy(*authCtx) {
		return nil, trace.AccessDenied("only remote proxy identities can replicate validated MFA challenges")
	}

	if err := checkReplicateValidatedMFAChallengeRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	currentCluster, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetTargetCluster() != currentCluster.GetClusterName() {
		return nil,
			trace.BadParameter(
				"target cluster %q does not match current cluster %q",
				req.GetTargetCluster(),
				currentCluster.GetClusterName(),
			)
	}

	chal := &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &types.Metadata{
			Name: req.GetName(),
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload:       req.GetPayload(),
			SourceCluster: req.GetSourceCluster(),
			TargetCluster: req.GetTargetCluster(),
			Username:      req.GetUsername(),
		},
	}

	created, err := s.storage.CreateValidatedMFAChallenge(ctx, req.GetTargetCluster(), chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &mfav1.ReplicateValidatedMFAChallengeResponse{
		ReplicatedChallenge: created,
	}, nil
}

func (s *Service) VerifyValidatedMFAChallenge(
	ctx context.Context,
	req *mfav1.VerifyValidatedMFAChallengeRequest,
) (*mfav1.VerifyValidatedMFAChallengeResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if b, ok := authCtx.Identity.(authz.BuiltinRole); !ok || (ok && !b.IsServer()) {
		return nil, trace.AccessDenied("only server identities can verify validated MFA challenges")
	}

	if err := checkVerifyValidatedMFAChallengeRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	currentCluster, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal, err := s.storage.GetValidatedMFAChallenge(ctx, currentCluster.GetClusterName(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case req.GetUsername() != chal.GetSpec().GetUsername():
		// Ensure the username in the request matches the username in the challenge to prevent replay attacks where an
		// attacker could use a validated challenge for one user to authenticate as a different user.
		return nil, trace.AccessDenied("request username does not match validated challenge username")

	case req.GetSourceCluster() != chal.GetSpec().GetSourceCluster():
		// Ensure the source cluster that was initially used to create the challenge matches the source cluster provided
		// in the request to prevent replay attacks where an attacker could use a validated challenge created in one
		// cluster to authenticate in a different cluster.
		return nil, trace.AccessDenied("request source cluster does not match validated challenge source cluster")
	}

	// Ensure the payload in the request matches the stored challenge payload for the same type.
	switch reqPayload := req.GetPayload().GetPayload().(type) {
	case *mfav1.SessionIdentifyingPayload_SshSessionId:
		storedSshSessionId := chal.GetSpec().GetPayload().GetSshSessionId()
		if !bytes.Equal(reqPayload.SshSessionId, storedSshSessionId) {
			return nil, trace.AccessDenied("request payload does not match validated challenge payload")
		}

	default:
		return nil, trace.AccessDenied("unsupported or mismatched payload type in request for this validated challenge")
	}

	return &mfav1.VerifyValidatedMFAChallengeResponse{}, nil
}

func validateCreateSessionChallengeRequest(req *mfav1.CreateSessionChallengeRequest) error {
	if err := checkPayload(req.GetPayload()); err != nil {
		return trace.Wrap(err)
	}

	// If either SSO challenge field is set, both must be set.
	switch {
	case req.SsoClientRedirectUrl != "" && req.ProxyAddressForSso == "":
		return trace.BadParameter("missing ProxyAddressForSso for SSO challenge")
	case req.SsoClientRedirectUrl == "" && req.ProxyAddressForSso != "":
		return trace.BadParameter("missing SsoClientRedirectUrl for SSO challenge")
	}

	return nil
}

func (s *Service) clusterExists(ctx context.Context, clusterName string) error {
	currentCluster, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if currentCluster.GetClusterName() == clusterName {
		return nil
	}

	if _, err := s.cache.GetRemoteCluster(ctx, clusterName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type devicesByType struct {
	Webauthn []*types.MFADevice
	SSO      *types.MFADevice
}

func (s *Service) groupAndFilterSupportedMFADevices(ctx context.Context, username string, devices []*types.MFADevice) devicesByType {
	var (
		webauthnDevices []*types.MFADevice
		ssoDevice       *types.MFADevice
	)

	// Only include supported device types for session-based MFA challenges. For example, TOTP devices are not supported
	// for session-based MFA and are therefore excluded.
	for _, dev := range devices {
		switch dev.Device.(type) {
		case *types.MFADevice_U2F, *types.MFADevice_Webauthn:
			webauthnDevices = append(webauthnDevices, dev)
		case *types.MFADevice_Sso:
			if ssoDevice == nil {
				ssoDevice = dev
			} else {
				// Currently only one SSO device is supported. In the future, we may support multiple SSO devices. If we
				// ever do, we'll need to update this logic to return all SSO devices instead of just the first one.
				s.logger.WarnContext(
					ctx,
					"Multiple SSO devices found for user, only the first device encountered will be used",
					"user", username,
					"used_device_id", ssoDevice.Id,
					"ignored_device_id", dev.Id,
				)
			}
		}
	}

	return devicesByType{
		Webauthn: webauthnDevices,
		SSO:      ssoDevice,
	}
}

func validateValidateSessionChallengeRequest(req *mfav1.ValidateSessionChallengeRequest) error {
	if req == nil {
		return trace.BadParameter("param ValidateSessionChallengeRequest is nil")
	}

	mfaResp := req.GetMfaResponse()
	if mfaResp == nil {
		return trace.BadParameter("nil ValidateSessionChallengeRequest.mfa_response")
	}

	if mfaResp.GetName() == "" {
		return trace.BadParameter("missing ValidateSessionChallengeRequest.mfa_response.name")
	}

	resp := mfaResp.GetResponse()
	if resp == nil {
		return trace.BadParameter("nil ValidateSessionChallengeRequest.mfa_response.response")
	}

	switch r := resp.(type) {
	case *mfav1.AuthenticateResponse_Webauthn:
		if r.Webauthn == nil {
			return trace.BadParameter("nil WebauthnResponse in AuthenticateResponse")
		}

	case *mfav1.AuthenticateResponse_Sso:
		if r.Sso == nil {
			return trace.BadParameter("nil SSOResponse in AuthenticateResponse")
		}

	default:
		return trace.BadParameter("unknown MFA response type %T", resp)
	}

	return nil
}

func (s *Service) validateWebauthnResponse(
	ctx context.Context,
	username string,
	resp *mfav1.AuthenticateResponse_Webauthn,
) (*authz.MFAAuthData, error) {
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

	// Convert to authz.MFAAuthData so that both validate functions return the same type for easier handling.
	return &authz.MFAAuthData{
		Device:        loginData.Device,
		User:          username,
		AllowReuse:    loginData.AllowReuse,
		Payload:       loginData.Payload,
		SourceCluster: loginData.SourceCluster,
		TargetCluster: loginData.TargetCluster,
	}, nil
}

func (s *Service) validateSSOResponse(
	ctx context.Context,
	username string,
	resp *mfav1.AuthenticateResponse_Sso,
) (*authz.MFAAuthData, error) {
	authData, err := s.authServer.VerifySSOMFASession(
		ctx,
		username,
		resp.Sso.RequestId,
		resp.Sso.Token,
		&mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION},
	)
	if err != nil {
		return nil, trace.AccessDenied("validate SSO response: %v", err)
	}

	return authData, nil
}

func (s *Service) emitValidationEvent(
	ctx context.Context,
	cluster string,
	username string,
	device *types.MFADevice,
	validateErr error,
) {
	event := &apievents.ValidateMFAAuthResponse{
		Metadata: apievents.Metadata{
			Type:        events.ValidateMFAAuthResponseEvent,
			ClusterName: cluster,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
		FlowType:     apievents.MFAFlowType_MFA_FLOW_TYPE_IN_BAND,
	}

	if validateErr != nil {
		event.Code = events.ValidateMFAAuthResponseFailureCode
		event.Success = false
		event.UserMessage = validateErr.Error()
		event.Error = validateErr.Error()
	} else {
		event.Code = events.ValidateMFAAuthResponseCode
		event.Success = true
		event.MFADevice = &apievents.MFADeviceMetadata{
			DeviceName: device.GetName(),
			DeviceID:   device.Id,
			DeviceType: device.MFAType(),
		}
	}

	if err := s.emitter.EmitAuditEvent(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "Failed to emit ValidateMFAAuthResponse event", "error", err)
		return
	}
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

func checkReplicateValidatedMFAChallengeRequest(req *mfav1.ReplicateValidatedMFAChallengeRequest) error {
	switch {
	case req.GetName() == "":
		return trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest name")

	case req.GetSourceCluster() == "":
		return trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest source_cluster")

	case req.GetTargetCluster() == "":
		return trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest target_cluster")

	case req.GetUsername() == "":
		return trace.BadParameter("missing ReplicateValidatedMFAChallengeRequest username")
	}

	if err := checkPayload(req.GetPayload()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func checkListValidatedMFAChallengesRequest(req *mfav1.ListValidatedMFAChallengesRequest) error {
	switch {
	case req.GetPageSize() <= 0:
		return trace.BadParameter("param ListValidatedMFAChallengesRequest.page_size must be a positive integer")
	}

	return nil
}

func checkVerifyValidatedMFAChallengeRequest(req *mfav1.VerifyValidatedMFAChallengeRequest) error {
	switch {
	case req.GetUsername() == "":
		return trace.BadParameter("missing VerifyValidatedMFAChallengeRequest username")

	case req.GetName() == "":
		return trace.BadParameter("missing VerifyValidatedMFAChallengeRequest name")
	}

	if err := checkPayload(req.GetPayload()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func checkPayload(sip *mfav1.SessionIdentifyingPayload) error {
	switch payload := sip.GetPayload().(type) {
	case *mfav1.SessionIdentifyingPayload_SshSessionId:
		if len(payload.SshSessionId) == 0 {
			return trace.BadParameter("empty SshSessionId in payload")
		}

	case nil:
		return trace.NotImplemented("missing or unsupported SessionIdentifyingPayload in request")

	default:
		return trace.BadParameter("unexpected SessionIdentifyingPayload type %T (this is a bug)", payload)
	}

	return nil
}

func isLocalProxy(authContext authz.Context) bool {
	if _, ok := authContext.UnmappedIdentity.(authz.BuiltinRole); !ok {
		return false
	}

	if !authContext.Checker.HasRole(string(types.RoleProxy)) {
		return false
	}

	return true
}

func isRemoteProxy(authContext authz.Context) bool {
	if _, ok := authContext.UnmappedIdentity.(authz.RemoteBuiltinRole); !ok {
		return false
	}

	if !authContext.Checker.HasRole(string(types.RoleRemoteProxy)) {
		return false
	}

	return true
}
