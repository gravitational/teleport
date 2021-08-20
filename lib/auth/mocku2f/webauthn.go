package mocku2f

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/trace"
)

// collectedClientData is part of the data signed by authenticators (after
// marshaled to JSON, hashed and appended to authData).
// https://www.w3.org/TR/webauthn-2/#dictionary-client-data
type collectedClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

// SignAssertion signs a WebAuthn assertion following the
// U2F-compat-getAssertion algorithm.
func (muk *Key) SignAssertion(origin string, assertion *webauthn.CredentialAssertion) (*webauthn.CredentialAssertionResponse, error) {
	// Is our credential allowed?
	ok := false
	for _, c := range assertion.Response.AllowedCredentials {
		if bytes.Equal(c.CredentialID, muk.KeyHandle) {
			ok = true
			break
		}
	}
	if !ok {
		return nil, trace.Errorf("device not allowed")
	}

	// Is the U2F app ID present?
	appID := assertion.Response.Extensions["appid"].(string)
	if appID == "" {
		return nil, trace.Errorf("missing u2f app ID")
	}
	appIDHash := sha256.Sum256([]byte(appID))

	// Marshal and hash collectedClientData - the result is what gets signed,
	// after appended to authData.
	ccd, err := json.Marshal(&collectedClientData{
		Type:      "webauthn.get",
		Challenge: base64.RawURLEncoding.EncodeToString(assertion.Response.Challenge),
		Origin:    origin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccd)

	res, err := muk.signAuthn(appIDHash[:], ccdHash[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &webauthn.CredentialAssertionResponse{
		PublicKeyCredential: protocol.PublicKeyCredential{
			Credential: protocol.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(muk.KeyHandle),
				Type: "public-key",
			},
			RawID: muk.KeyHandle,
			Extensions: protocol.AuthenticationExtensionsClientOutputs{
				"appid": true, // U2F App ID used.
			},
		},
		AssertionResponse: protocol.AuthenticatorAssertionResponse{
			AuthenticatorResponse: protocol.AuthenticatorResponse{
				ClientDataJSON: ccd,
			},
			AuthenticatorData: res.AuthData,
			// Signature starts after user presence (1byte) and counter (4 bytes).
			Signature: res.SignData[5:],
		},
	}, nil
}
