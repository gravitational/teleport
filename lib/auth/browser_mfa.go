package auth

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/trace"
)

// EncryptBrowserMFAResponse encrypts a browser MFA webauthn response and returns the redirect URL with the encrypted response.
func encryptBrowserMFAResponse(redirectURL *url.URL, webauthnResponse *wantypes.CredentialAssertionResponse) (string, error) {
	consoleResponse := authclient.SSHLoginResponse{
		BrowserMFAWebauthnResponse: webauthnResponse,
	}
	out, err := json.Marshal(consoleResponse)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Extract secret out of the redirect URL.
	secretKey := redirectURL.Query().Get("secret_key")
	if secretKey == "" {
		return "", trace.BadParameter("missing secret_key")
	}

	// AES-GCM based symmetric cipher.
	key, err := secret.ParseKey([]byte(secretKey))
	if err != nil {
		return "", trace.Wrap(err)
	}
	ciphertext, err := key.Seal(out)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Place ciphertext into the redirect URL.
	redirectURL.RawQuery = url.Values{"response": {string(ciphertext)}}.Encode()

	return redirectURL.String(), nil
}

// ValidateBrowserMFAChallenge validates an MFA challenge response and returns the redirect URL with encrypted response.
func (a *Server) ValidateBrowserMFAChallenge(ctx context.Context, requestID string, webauthnResponse *wantypes.CredentialAssertionResponse) (string, error) {
	mfaSession, err := a.GetSSOMFASession(ctx, requestID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	u, err := url.Parse(mfaSession.ClientRedirectURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	clientRedirectURL, err := encryptBrowserMFAResponse(u, webauthnResponse)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientRedirectURL, nil
}
