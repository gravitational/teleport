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

	"github.com/gravitational/trace"

	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/internal/browsermfa"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
)

// CompleteBrowserMFAChallenge completes an MFA challenge response by returning the redirect URL with encrypted response.
func (a *Server) CompleteBrowserMFAChallenge(ctx context.Context, requestID string, webauthnResponse *webauthnpb.CredentialAssertionResponse) (string, error) {
	const notFoundErrMsg = "mfa session data not found"
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
	clientRedirectURL, err := browsermfa.EncryptBrowserMFAResponse(u, wr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientRedirectURL, nil
}
