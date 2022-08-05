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
	"encoding/binary"
	"encoding/json"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/fxamacker/cbor/v2"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

// SignAssertion signs a WebAuthn assertion following the
// U2F-compat-getAssertion algorithm.
func (muk *Key) SignAssertion(origin string, assertion *wanlib.CredentialAssertion) (*wanlib.CredentialAssertionResponse, error) {
	// Reference:
	// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#u2f-authenticatorGetAssertion-interoperability

	// Is our credential allowed?
	ok := false
	for _, c := range assertion.Response.AllowedCredentials {
		if bytes.Equal(c.CredentialID, muk.KeyHandle) {
			ok = true
			break
		}
	}
	if !ok && !muk.IgnoreAllowedCredentials {
		return nil, trace.BadParameter("device not allowed")
	}

	// Use RPID or App ID?
	appID := assertion.Response.RelyingPartyID
	if value, ok := assertion.Response.Extensions[wanlib.AppIDExtension]; !muk.PreferRPID && ok {
		if appID, ok = value.(string); !ok {
			return nil, trace.BadParameter("u2f app ID has unexpected type: %T", value)
		}
	}
	appIDHash := sha256.Sum256([]byte(appID))

	// Marshal and hash collectedClientData - the result is what gets signed,
	// after appended to authData.
	ccd, err := json.Marshal(&wancli.CollectedClientData{
		Type:      string(protocol.AssertCeremony),
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
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: muk.KeyHandle,
			// Mimic browsers and don't set the output AppID extension, even if we
			// used it.
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

// SignCredentialCreation signs a WebAuthn credential creation request following
// the U2F-compat-makeCredential algorithm.
func (muk *Key) SignCredentialCreation(origin string, cc *wanlib.CredentialCreation) (*wanlib.CredentialCreationResponse, error) {
	// Reference:
	// https: // fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#fig-u2f-compat-makeCredential

	// Is our credential allowed?
	for _, cred := range cc.Response.CredentialExcludeList {
		if bytes.Equal(cred.CredentialID, muk.KeyHandle) {
			return nil, trace.BadParameter("credential in exclude list")
		}
	}

	// Is our algorithm allowed?
	ok := false
	for _, params := range cc.Response.Parameters {
		if params.Type == protocol.PublicKeyCredentialType && params.Algorithm == webauthncose.AlgES256 {
			ok = true
			break
		}
	}
	if !ok {
		return nil, trace.BadParameter("ES256 not allowed by credential parameters")
	}
	// Can we fulfill the authenticator selection?
	if aa := cc.Response.AuthenticatorSelection.AuthenticatorAttachment; aa == protocol.Platform {
		return nil, trace.BadParameter("platform attachment required by authenticator selection")
	}
	if rrk := cc.Response.AuthenticatorSelection.RequireResidentKey; rrk != nil && *rrk {
		return nil, trace.BadParameter("resident key required by authenticator selection")
	}
	if uv := cc.Response.AuthenticatorSelection.UserVerification; uv == protocol.VerificationRequired {
		return nil, trace.BadParameter("user verification required by authenticator selection")
	}

	ccd, err := json.Marshal(&wancli.CollectedClientData{
		Type:      string(protocol.CreateCeremony),
		Challenge: base64.RawURLEncoding.EncodeToString(cc.Response.Challenge),
		Origin:    origin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccd)
	appIDHash := sha256.Sum256([]byte(cc.Response.RelyingParty.ID))

	res, err := muk.signRegister(appIDHash[:], ccdHash[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubKeyCBOR, err := wanlib.U2FKeyToCBOR(&muk.PrivateKey.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authData := &bytes.Buffer{}
	authData.Write(appIDHash[:])
	// Attested credential data present.
	// https://www.w3.org/TR/webauthn-2/#attested-credential-data.
	authData.WriteByte(byte(protocol.FlagAttestedCredentialData | protocol.FlagUserPresent))
	binary.Write(authData, binary.BigEndian, uint32(0)) // counter, zeroed
	authData.Write(make([]byte, 16))                    // AAGUID, zeroed
	binary.Write(authData, binary.BigEndian, uint16(len(muk.KeyHandle)))
	authData.Write(muk.KeyHandle)
	authData.Write(pubKeyCBOR)

	attObj, err := cbor.Marshal(&protocol.AttestationObject{
		RawAuthData: authData.Bytes(),
		// See https://www.w3.org/TR/webauthn-2/#sctn-fido-u2f-attestation.
		Format: "fido-u2f",
		AttStatement: map[string]interface{}{
			"sig": res.Signature,
			"x5c": []interface{}{muk.Cert},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &wanlib.CredentialCreationResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(muk.KeyHandle),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: muk.KeyHandle,
		},
		AttestationResponse: wanlib.AuthenticatorAttestationResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: ccd,
			},
			AttestationObject: attObj,
		},
	}, nil
}
