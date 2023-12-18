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

package webauthncli

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/flynn/u2f/u2ftoken"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// U2FRegister implements Register for U2F/CTAP1 devices.
// The implementation is backed exclusively by Go code, making it useful in
// scenarios where libfido2 is unavailable.
func U2FRegister(ctx context.Context, origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	// Preliminary checks, more below.
	switch {
	case origin == "":
		return nil, trace.BadParameter("origin required")
	case cc == nil:
		return nil, trace.BadParameter("credential creation required")
	case cc.Response.RelyingParty.ID == "":
		return nil, trace.BadParameter("credential creation missing relying party ID")
	}

	// U2F/CTAP1 is limited to ES256, check if it's allowed.
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

	// Prepare challenge data for the device.
	ccdJSON, err := json.Marshal(&CollectedClientData{
		Type:      string(protocol.CreateCeremony),
		Challenge: base64.RawURLEncoding.EncodeToString(cc.Response.Challenge),
		Origin:    origin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccdJSON)
	rpIDHash := sha256.Sum256([]byte(cc.Response.RelyingParty.ID))

	var appIDHash []byte
	if value, ok := cc.Response.Extensions[wantypes.AppIDExtension]; ok {
		appID := fmt.Sprint(value)
		h := sha256.Sum256([]byte(appID))
		appIDHash = h[:]
	}

	// Register!
	var rawResp []byte
	if err := RunOnU2FDevices(ctx, func(t Token) error {
		// Is the authenticator in the credential exclude list?
		for _, cred := range cc.Response.CredentialExcludeList {
			for _, app := range [][]byte{rpIDHash[:], appIDHash} {
				if len(app) == 0 {
					continue
				}

				// Check if the device is already registered by calling
				// CheckAuthenticate.
				// If the method succeeds then the device knows about the
				// {key handle, app} pair, which means it is already registered.
				// CheckAuthenticate doesn't require user interaction.
				if err := t.CheckAuthenticate(u2ftoken.AuthenticateRequest{
					Challenge:   ccdHash[:],
					Application: app,
					KeyHandle:   cred.CredentialID,
				}); err == nil {
					log.Warnf(
						"WebAuthn: Authenticator already registered under credential ID %q",
						base64.RawURLEncoding.EncodeToString(cred.CredentialID))
					return ErrAlreadyRegistered // Remove authenticator from list
				}
			}
		}

		var err error
		rawResp, err = t.Register(u2ftoken.RegisterRequest{
			Challenge:   ccdHash[:],
			Application: rpIDHash[:],
		})
		return err
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse U2F response and convert to Webauthn - after that we are done.
	resp, err := parseU2FRegistrationResponse(rawResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccr, err := credentialResponseFromU2F(ccdJSON, rpIDHash[:], resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
		},
	}, nil
}

type u2fRegistrationResponse struct {
	PubKey                                *ecdsa.PublicKey
	KeyHandle, AttestationCert, Signature []byte
}

func parseU2FRegistrationResponse(resp []byte) (*u2fRegistrationResponse, error) {
	// Reference:
	// https://fidoalliance.org/specs/fido-u2f-v1.2-ps-20170411/fido-u2f-raw-message-formats-v1.2-ps-20170411.html#registration-response-message-success

	// minRespLen is based on:
	// 1 byte reserved +
	// 65 pubKey +
	// 1 key handle length +
	// N key handle (at least 1) +
	// N attestation cert (at least 1, need to parse to find out) +
	// N signature (at least 1, spec says 71-73 bytes, YMMV)
	const pubKeyLen = 65
	const minRespLen = 1 + pubKeyLen + 4
	if len(resp) < minRespLen {
		return nil, trace.BadParameter("U2F response too small, got %v bytes, expected at least %v", len(resp), minRespLen)
	}

	// Reads until the key handle length are guaranteed by the size checking
	// above.
	buf := resp
	if buf[0] != 0x05 {
		return nil, trace.BadParameter("invalid reserved byte: %v", buf[0])
	}
	buf = buf[1:]

	// public key, "4||X||Y" form.
	pubKeyBytes := buf[:pubKeyLen]
	// Validate pubKey points.
	if _, err := ecdh.P256().NewPublicKey(pubKeyBytes); err != nil {
		return nil, trace.Wrap(err, "unmarshal public key")
	}
	// There's no API to pry away X and Y from ecdh.PublicKey, so we do it
	// manually, but only after the key is validated.
	const uncompressedForm = 4
	if pubKeyBytes[0] != uncompressedForm {
		return nil, trace.BadParameter("public key not in uncompressed form")
	}
	pubKeyBytes = pubKeyBytes[1:] // holds X||Y
	l := len(pubKeyBytes) / 2     // holds the size of a coordinate (X or Y)
	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(pubKeyBytes[:l]),
		Y:     new(big.Int).SetBytes(pubKeyBytes[l:]),
	}
	buf = buf[pubKeyLen:]

	// key handle
	l = int(buf[0]) // holds the keyHandle length.
	buf = buf[1:]
	// Size checking resumed from now on.
	if len(buf) < l {
		return nil, trace.BadParameter("key handle length is %v, but only %v bytes are left", l, len(buf))
	}
	keyHandle := buf[:l]
	buf = buf[l:]

	// Parse the certificate to figure out its size, then call
	// x509.ParseCertificate with a correctly-sized byte slice.
	sig, err := asn1.Unmarshal(buf, &asn1.RawValue{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the cert to check that it is valid - we don't actually need the
	// parsed cert after it is proved to be well-formed.
	attestationCert := buf[:len(buf)-len(sig)]
	if _, err := x509.ParseCertificate(attestationCert); err != nil {
		return nil, trace.Wrap(err)
	}

	return &u2fRegistrationResponse{
		PubKey:          pubKey,
		KeyHandle:       keyHandle,
		AttestationCert: attestationCert,
		Signature:       sig,
	}, nil
}

func credentialResponseFromU2F(ccdJSON, appIDHash []byte, resp *u2fRegistrationResponse) (*wantypes.CredentialCreationResponse, error) {
	// Reference:
	// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#fig-u2f-compat-makeCredential

	pubKeyCBOR, err := wanlib.U2FKeyToCBOR(resp.PubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Assemble authenticator data.
	authData := &bytes.Buffer{}
	authData.Write(appIDHash[:])
	// Attested credential data present.
	// https://www.w3.org/TR/webauthn-2/#attested-credential-data.
	authData.WriteByte(byte(protocol.FlagAttestedCredentialData | protocol.FlagUserPresent))
	binary.Write(authData, binary.BigEndian, uint32(0))                   // counter, zeroed
	authData.Write(make([]byte, 16))                                      // AAGUID, zeroed
	binary.Write(authData, binary.BigEndian, uint16(len(resp.KeyHandle))) // L
	authData.Write(resp.KeyHandle)
	authData.Write(pubKeyCBOR)

	// Assemble attestation object
	attestationObj, err := cbor.Marshal(&protocol.AttestationObject{
		RawAuthData: authData.Bytes(),
		// See https://www.w3.org/TR/webauthn-2/#sctn-fido-u2f-attestation.
		Format: "fido-u2f",
		AttStatement: map[string]interface{}{
			"sig": resp.Signature,
			"x5c": []interface{}{resp.AttestationCert},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &wantypes.CredentialCreationResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(resp.KeyHandle),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: resp.KeyHandle,
		},
		AttestationResponse: wantypes.AuthenticatorAttestationResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: ccdJSON,
			},
			AttestationObject: attestationObj,
		},
	}, nil
}
