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

package auth

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/internal"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/trace"
)

// BeginBrowserMFAChallenge creates a new Browser MFA auth request and session
// data for the given user stored in an SSO MFA record
func (a *Server) BeginBrowserMFAChallenge(ctx context.Context, params mfatypes.BeginBrowserMFAChallengeParams) (*proto.BrowserMFAChallenge, error) {
	if err := sso.ValidateClientRedirect(params.BrowserMFATSHRedirectURL, sso.CeremonyTypeMFA, nil); err != nil {
		return nil, trace.Wrap(err, InvalidClientRedirectErrorMessage)
	}

	requestID := uuid.NewString()
	browserChal := &proto.BrowserMFAChallenge{
		RequestId: requestID,
	}

	if err := a.upsertSSOMFASession(ctx, params.User, requestID, constants.Browser /* connectorId */, constants.Browser /* connectorType */, params.BrowserMFATSHRedirectURL, params.Ext, params.SIP, params.SourceCluster, params.TargetCluster); err != nil {
		return nil, trace.Wrap(err)
	}

	return browserChal, nil
}

// CompleteBrowserMFAChallenge completes an MFA challenge response by returning the redirect URL with encrypted response.
func (a *Server) CompleteBrowserMFAChallenge(ctx context.Context, requestID string, webauthnResponse *webauthnpb.CredentialAssertionResponse) (string, error) {
	const notFoundErrMsg = "mfa sso session data not found"

	// Retrieve the MFA session
	mfaSession, err := a.GetSSOMFASession(ctx, requestID)
	if trace.IsNotFound(err) {
		return "", trace.AccessDenied("%s", notFoundErrMsg)
	} else if err != nil {
		return "", trace.Wrap(err)
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if mfaSession.Username != user.GetIdentity().Username {
		return "", trace.AccessDenied("%s", notFoundErrMsg)
	}

	// Valid WebAuthn response, encrypt and return it
	u, err := url.Parse(mfaSession.ClientRedirectURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	wr := wantypes.CredentialAssertionResponseFromProto(webauthnResponse)
	clientRedirectURL, err := internal.EncryptBrowserMFAResponse(u, wr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientRedirectURL, nil
}

// VerifyBrowserMFASession verifies that the given Browser MFA webauthn response matches an existing MFA session
// for the user and session ID. It also checks the required extensions, and finishes by deleting
// the MFA session if reuse is not allowed.
func (a *Server) VerifyBrowserMFASession(ctx context.Context, username, sessionID string, webauthnResponse *webauthnpb.CredentialAssertionResponse, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
	if requiredExtensions == nil {
		return nil, trace.BadParameter("requested challenge extensions must be supplied.")
	}

	const notFoundErrMsg = "browser mfa session data not found"
	mfaSess, err := a.GetSSOMFASessionData(ctx, sessionID)
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("%s", notFoundErrMsg)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify the user's name matches.
	if mfaSess.Username != username {
		return nil, trace.NotFound("%s", notFoundErrMsg)
	}

	// Check if the MFA session matches the user's Browser MFA settings.
	devs, err := a.Services.GetMFADevices(ctx, username, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check the user has a Browser MFA device
	groupedDevs := groupByDeviceType(devs)
	if groupedDevs.Browser == nil {
		a.logger.DebugContext(ctx,
			"Browser MFA failed. It isn't available when a user has no WebAuthn devices or has SSO MFA",
			"webauthn_devices", len(groupedDevs.Webauthn),
			"sso_mfa_available", groupedDevs.SSO != nil,
			"user", username,
		)

		var cause []string
		if len(groupedDevs.Webauthn) == 0 {
			cause = append(cause, "no webauthn devices are registered")
		}
		if groupedDevs.SSO != nil {
			cause = append(cause, "SSO MFA is available (preferred over Browser MFA)")
		}
		accessDeniedMsg := "user has no browser mfa device available"
		if len(cause) > 0 {
			accessDeniedMsg += fmt.Sprintf(" because %s", strings.Join(cause, " and "))
		}
		return nil, trace.AccessDenied("%s", accessDeniedMsg)
	}

	// Check if the given scope is satisfied by the challenge scope.
	if requiredExtensions.Scope != mfaSess.ChallengeExtensions.Scope {
		return nil, trace.AccessDenied("required scope %q is not satisfied by the given browser mfa session with scope %q", requiredExtensions.Scope, mfaSess.ChallengeExtensions.Scope)
	}

	// If this session is reusable, but this context forbids reusable sessions, return an error.
	if requiredExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO && mfaSess.ChallengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		return nil, trace.AccessDenied("the given browser mfa session allows reuse, but reuse is not permitted in this context")
	}

	// Convert from protobuf type to wantypes
	wanResp := wantypes.CredentialAssertionResponseFromProto(webauthnResponse)

	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	waConfig, err := cap.GetWebauthn()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginFlow := &wanlib.LoginFlow{
		Webauthn: waConfig,
		Identity: a.Services,
	}

	// Verify the webauthn response against the original challenge scope.
	// This validates the cryptographic signature and challenge match.
	loginData, err := loginFlow.Finish(ctx, username, wanResp, &mfav1.ChallengeExtensions{
		Scope:      mfaSess.ChallengeExtensions.Scope,
		AllowReuse: mfaSess.ChallengeExtensions.AllowReuse,
	})
	if err != nil {
		return nil, trace.AccessDenied("verify WebAuthn response: %v", err)
	}

	if mfaSess.ChallengeExtensions.AllowReuse != mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		if err := a.DeleteSSOMFASessionData(ctx, sessionID); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &authz.MFAAuthData{
		Device:        loginData.Device,
		User:          username,
		AllowReuse:    mfaSess.ChallengeExtensions.AllowReuse,
		Payload:       mfaSess.Payload,
		SourceCluster: mfaSess.SourceCluster,
		TargetCluster: mfaSess.TargetCluster,
		MFAViaBrowser: true,
	}, nil
}
