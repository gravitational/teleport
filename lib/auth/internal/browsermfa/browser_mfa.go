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

package browsermfa

import (
	"encoding/json"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/secret"
)

// Payload type required by (lib/client/sso/redirector.go).
type ssoRedirectorResponse = authclient.CLILoginResponse

// EncryptBrowserMFAResponse encrypts a browser MFA webauthn response and returns the redirect URL with the encrypted response.
func EncryptBrowserMFAResponse(redirectURL *url.URL, webauthnResponse *wantypes.CredentialAssertionResponse) (string, error) {
	// Extract secret out of the redirect URL.
	secretKey := redirectURL.Query().Get("secret_key")
	if secretKey == "" {
		return "", trace.BadParameter("missing secret_key")
	}

	// AES-GCM based symmetric cipher.
	key, err := secret.ParseKey([]byte(secretKey))
	if err != nil {
		return "", trace.Wrap(err, "parse secret key")
	}

	// Build response payload.
	consoleResponse := ssoRedirectorResponse{
		BrowserMFAWebauthnResponse: webauthnResponse,
	}
	out, err := json.Marshal(consoleResponse)
	if err != nil {
		return "", trace.Wrap(err, "marshaling response payload")
	}

	// Base64 and encrypt the response payload.
	ciphertext, err := key.Seal(out)
	if err != nil {
		return "", trace.Wrap(err, "seal response with secret key")
	}

	// Place ciphertext into the redirect URL.
	redirectURL.RawQuery = url.Values{"response": {string(ciphertext)}}.Encode()

	return redirectURL.String(), nil
}
