/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package mocku2f

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// SignAssertion signs a WebAuthn assertion following the
// U2F-compat-getAssertion algorithm.
func (muk *Key) SignAssertion(origin string, assertion *wantypes.CredentialAssertion) (*wantypes.CredentialAssertionResponse, error) {
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
	if value, ok := assertion.Response.Extensions[wantypes.AppIDExtension]; !muk.PreferRPID && ok {
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

	// If passwordless, then relay our WebAuthn UserHandle.
	var userHandle []byte
	if len(assertion.Response.AllowedCredentials) == 0 && len(muk.UserHandle) > 0 {
		userHandle = muk.UserHandle
	}

	return &wantypes.CredentialAssertionResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(muk.KeyHandle),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: muk.KeyHandle,
			// Mimic browsers and don't set the output AppID extension, even if we
			// used it.
		},
		AssertionResponse: wantypes.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: ccd,
			},
			AuthenticatorData: res.AuthData,
			// Signature starts after user presence (1byte) and counter (4 bytes).
			Signature:  res.SignData[5:],
			UserHandle: userHandle,
		},
	}, nil
}

// SignCredentialCreation signs a WebAuthn credential creation request following
// the U2F-compat-makeCredential algorithm.
func (muk *Key) SignCredentialCreation(origin string, cc *wantypes.CredentialCreation) (*wantypes.CredentialCreationResponse, error) {
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
	rrk, err := cc.RequireResidentKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rrk && !muk.AllowResidentKey {
		return nil, trace.BadParameter("resident key required by authenticator selection")
	}
	if uv := cc.Response.AuthenticatorSelection.UserVerification; uv == protocol.VerificationRequired && !muk.SetUV {
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
	flags := res.RawResp[0]
	if flags == u2fRegistrationFlags {
		// Apply U2F-compabitle authenticatorMakeCredential logic.
		// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#u2f-authenticatorMakeCredential-interoperability
		flags = byte(protocol.FlagUserPresent | protocol.FlagAttestedCredentialData)
	}

	pubKeyCBOR, err := wanlib.U2FKeyToCBOR(&muk.PrivateKey.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authData := &bytes.Buffer{}
	authData.Write(appIDHash[:])
	authData.WriteByte(flags)
	binary.Write(authData, binary.BigEndian, uint32(0)) // counter, zeroed
	authData.Write(make([]byte, 16))                    // AAGUID, zeroed
	binary.Write(authData, binary.BigEndian, uint16(len(muk.KeyHandle)))
	authData.Write(muk.KeyHandle)
	authData.Write(pubKeyCBOR)

	attObj, err := cbor.Marshal(&protocol.AttestationObject{
		RawAuthData: authData.Bytes(),
		// See https://www.w3.org/TR/webauthn-2/#sctn-fido-u2f-attestation.
		Format: "fido-u2f",
		AttStatement: map[string]any{
			"sig": res.Signature,
			"x5c": []any{muk.Cert},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Save the WebAuthn UserHandle if this is a resident key creation.
	if rrk && len(cc.Response.User.ID) > 0 && len(muk.UserHandle) == 0 {
		muk.UserHandle = cc.Response.User.ID
	}

	var exts *wantypes.AuthenticationExtensionsClientOutputs
	if muk.ReplyWithCredProps {
		exts = &wantypes.AuthenticationExtensionsClientOutputs{
			CredProps: &wantypes.CredentialPropertiesOutput{
				RK: true,
			},
		}
	}

	return &wantypes.CredentialCreationResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(muk.KeyHandle),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID:      muk.KeyHandle,
			Extensions: exts,
		},
		AttestationResponse: wantypes.AuthenticatorAttestationResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: ccd,
			},
			AttestationObject: attObj,
		},
	}, nil
}
