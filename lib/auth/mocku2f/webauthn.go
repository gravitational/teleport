/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mocku2f

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

// SignAssertion signs a WebAuthn assertion following the
// U2F-compat-getAssertion algorithm.
func (muk *Key) SignAssertion(origin string, assertion *wanlib.CredentialAssertion) (*wanlib.CredentialAssertionResponse, error) {
	// Is our credential allowed?
	ok := false
	for _, c := range assertion.Response.AllowedCredentials {
		if bytes.Equal(c.CredentialID, muk.KeyHandle) {
			ok = true
			break
		}
	}
	if !ok {
		return nil, trace.BadParameter("device not allowed")
	}

	// Is the U2F app ID present?
	value, ok := assertion.Response.Extensions[wanlib.AppIDExtension]
	if !ok {
		return nil, trace.BadParameter("missing u2f app ID")
	}
	appID, ok := value.(string)
	if !ok {
		return nil, trace.BadParameter("u2f app ID has unexpected type: %T", value)
	}
	appIDHash := sha256.Sum256([]byte(appID))

	// Marshal and hash collectedClientData - the result is what gets signed,
	// after appended to authData.
	ccd, err := json.Marshal(&wancli.CollectedClientData{
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

	return &wanlib.CredentialAssertionResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(muk.KeyHandle),
				Type: "public-key",
			},
			RawID: muk.KeyHandle,
			Extensions: &wanlib.AuthenticationExtensionsClientOutputs{
				AppID: true, // U2F App ID used.
			},
		},
		AssertionResponse: wanlib.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: ccd,
			},
			AuthenticatorData: res.AuthData,
			// Signature starts after user presence (1byte) and counter (4 bytes).
			Signature: res.SignData[5:],
		},
	}, nil
}
