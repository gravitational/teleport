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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/internal/browsermfa"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/services"
)

// CompleteBrowserMFAChallenge completes an MFA challenge response by returning the redirect URL with encrypted response.
func (a *Server) CompleteBrowserMFAChallenge(ctx context.Context, requestID string, webauthnResponse *webauthnpb.CredentialAssertionResponse) (string, error) {
	const notFoundErrMsg = "mfa session data not found"
	// Retrieve the MFA session
	mfaSession, err := a.GetMFASession(ctx, requestID)
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
	u, err := url.Parse(mfaSession.TSHRedirectURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	wr := wantypes.CredentialAssertionResponseFromProto(webauthnResponse)
	clientRedirectURL, err := browsermfa.EncryptBrowserMFAResponse(u, wr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientRedirectURL, nil
}

// BeginBrowserMFAChallenge creates a new Browser MFA auth request and session
// data for the given params and stores it in the backend.
func (a *Server) BeginBrowserMFAChallenge(ctx context.Context, params mfatypes.BeginBrowserMFAChallengeParams) (*proto.BrowserMFAChallenge, error) {
	if err := sso.ValidateClientRedirect(params.BrowserMFATSHRedirectURL, sso.CeremonyTypeMFA, nil); err != nil {
		return nil, trace.Wrap(err, InvalidClientRedirectErrorMessage)
	}

	requestID := uuid.NewString()
	browserChal := &proto.BrowserMFAChallenge{
		RequestId: requestID,
	}

	sessionData := &services.MFASessionData{
		Username:       params.User,
		RequestID:      requestID,
		ConnectorID:    constants.BrowserMFA,
		ConnectorType:  constants.BrowserMFA,
		TSHRedirectURL: params.BrowserMFATSHRedirectURL,
		ChallengeExtensions: &mfatypes.ChallengeExtensions{
			Scope:                       params.Ext.Scope,
			AllowReuse:                  params.Ext.AllowReuse,
			UserVerificationRequirement: params.Ext.UserVerificationRequirement,
		},
	}

	if err := a.UpsertMFASessionData(ctx, sessionData); err != nil {
		return nil, trace.Wrap(err)
	}

	return browserChal, nil
}
