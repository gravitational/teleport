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
	"net/url"

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

	proxyAddr := params.ProxyAddress
	if proxyAddr == "" {
		proxyAddr = a.getProxyPublicAddr(ctx)
	}
	if proxyAddr == "" {
		return nil, trace.BadParameter("proxy address not available for browser MFA")
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

// ValidateBrowserMFAChallenge validates an MFA challenge response and returns the redirect URL with encrypted response.
func (a *Server) ValidateBrowserMFAChallenge(ctx context.Context, requestID string, webauthnResponse *webauthnpb.CredentialAssertionResponse) (string, error) {
	// Retrieve the MFA session
	mfaSession, err := a.GetSSOMFASession(ctx, requestID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Get WebAuthn configuration for validation
	pref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	webConfig, err := pref.GetWebauthn()
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Validate the WebAuthn response
	webLogin := &wanlib.LoginFlow{
		Webauthn: webConfig,
		Identity: a.Services,
	}

	wr := wantypes.CredentialAssertionResponseFromProto(webauthnResponse)
	if err := webLogin.Validate(ctx,
		mfaSession.Username,
		wr,
		&mfav1.ChallengeExtensions{
			Scope:                       mfaSession.ChallengeExtensions.Scope,
			AllowReuse:                  mfaSession.ChallengeExtensions.AllowReuse,
			UserVerificationRequirement: mfaSession.ChallengeExtensions.UserVerificationRequirement,
		},
	); err != nil {
		return "", trace.Wrap(err, "failed to validate browser MFA response")
	}

	// Valid WebAuthn response, encrypt and return it
	u, err := url.Parse(mfaSession.ClientRedirectURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	clientRedirectURL, err := internal.EncryptBrowserMFAResponse(u, wr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientRedirectURL, nil
}

// VerifyBrowserMFASession verifies that the given browser mfa webauthn response matches an existing MFA session
// for the user and session ID. It also checks the required extensions, and finishes by deleting
// the MFA session if reuse is not allowed.
func (a *Server) VerifyBrowserMFASession(ctx context.Context, username, sessionID string, webauthnResponse *webauthnpb.CredentialAssertionResponse, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
	if requiredExtensions == nil {
		return nil, trace.BadParameter("requested challenge extensions must be supplied.")
	}

	const notFoundErrMsg = "mfa browser session data not found"
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

	groupedDevs := groupByDeviceType(devs)
	if groupedDevs.Browser == nil {
		return nil, trace.AccessDenied("invalid browser mfa session data; user has no browser mfa device available")
	}

	// Verify the webauthn response.
	pref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the user's WebAuthn preference. If it doesn't exist, continue since WebAuthn may not be enabled.
	webConfig, err := pref.GetWebauthn()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	webLogin := &wanlib.LoginFlow{
		Webauthn: webConfig,
		Identity: a.Services,
	}

	// Convert from protobuf type to wantypes
	wanResp := wantypes.CredentialAssertionResponseFromProto(webauthnResponse)

	// Verify the webauthn response against the original challenge scope.
	// This validates the cryptographic signature and challenge match.
	loginData, err := webLogin.Finish(ctx, username, wanResp, &mfav1.ChallengeExtensions{
		Scope:      mfaSess.ChallengeExtensions.Scope,
		AllowReuse: mfaSess.ChallengeExtensions.AllowReuse,
	})
	if err != nil {
		return nil, trace.AccessDenied("verify WebAuthn response: %v", err)
	}

	// Check if the given scope is satisfied by the challenge scope.
	if requiredExtensions.Scope != mfaSess.ChallengeExtensions.Scope {
		return nil, trace.AccessDenied("required scope %q is not satisfied by the given browser mfa session with scope %q", requiredExtensions.Scope, mfaSess.ChallengeExtensions.Scope)
	}

	// If this session is reusable, but this context forbids reusable sessions, return an error.
	if requiredExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO && mfaSess.ChallengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		return nil, trace.AccessDenied("the given browser mfa session allows reuse, but reuse is not permitted in this context")
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
	}, nil
}
