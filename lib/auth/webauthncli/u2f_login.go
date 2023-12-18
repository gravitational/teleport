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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/flynn/u2f/u2ftoken"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// U2FLogin implements Login for U2F/CTAP1 devices.
// The implementation is backed exclusively by Go code, making it useful in
// scenarios where libfido2 is unavailable.
func U2FLogin(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
	switch {
	case origin == "":
		return nil, trace.BadParameter("origin required")
	case assertion == nil:
		return nil, trace.BadParameter("assertion required")
	case len(assertion.Response.AllowedCredentials) == 0 &&
		assertion.Response.UserVerification == protocol.VerificationRequired:
		return nil, trace.BadParameter("Passwordless not supported in U2F mode. Please install a recent version of tsh.")
	case len(assertion.Response.Challenge) == 0:
		return nil, trace.BadParameter("assertion challenge required")
	case assertion.Response.RelyingPartyID == "":
		return nil, trace.BadParameter("assertion RPID required")
	case len(assertion.Response.AllowedCredentials) == 0:
		return nil, trace.BadParameter("assertion has no allowed credentials")
	case assertion.Response.UserVerification == protocol.VerificationRequired:
		return nil, trace.BadParameter(
			"assertion required user verification, but it cannot be guaranteed under CTAP1")
	}

	// References:
	// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#u2f-authenticatorGetAssertion-interoperability
	// https://www.w3.org/TR/webauthn-2/#sctn-op-get-assertion

	ccdJSON, err := json.Marshal(&CollectedClientData{
		Type:      string(protocol.AssertCeremony),
		Challenge: base64.RawURLEncoding.EncodeToString(assertion.Response.Challenge),
		Origin:    origin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccdJSON)
	rpID := assertion.Response.RelyingPartyID
	rpIDHash := sha256.Sum256([]byte(rpID))

	// Did we get the App ID extension?
	var appID string
	var appIDHash [32]byte
	if value, ok := assertion.Response.Extensions[wantypes.AppIDExtension]; ok {
		appID = fmt.Sprint(value)
		appIDHash = sha256.Sum256([]byte(appID))
	}

	// Variables below are filled by the callback on success.
	var authCred wantypes.CredentialDescriptor
	var authResp *u2ftoken.AuthenticateResponse
	var usedAppID bool
	makeAuthU2F := func(cred wantypes.CredentialDescriptor, req u2ftoken.AuthenticateRequest, appID bool) func(Token) error {
		return func(token Token) error {
			if err := token.CheckAuthenticate(req); err != nil {
				return err // don't wrap, inspected by RunOnU2FDevices
			}
			resp, err := token.Authenticate(req)
			if err != nil {
				return err // don't wrap, inspected by RunOnU2FDevices
			}
			authCred = cred
			authResp = resp
			usedAppID = appID
			return nil
		}
	}

	// Assemble credential+RPID pairs to attempt.
	var fns []func(Token) error
	for _, cred := range assertion.Response.AllowedCredentials {
		req := u2ftoken.AuthenticateRequest{
			Challenge:   ccdHash[:],
			Application: rpIDHash[:],
			KeyHandle:   cred.CredentialID,
		}
		fns = append(fns, makeAuthU2F(cred, req, false /* appID */))
		if appID != "" {
			req.Application = appIDHash[:]
			fns = append(fns, makeAuthU2F(cred, req, true /* appID */))
		}
	}

	// Run!
	if err := RunOnU2FDevices(ctx, fns...); err != nil {
		return nil, trace.Wrap(err)
	}

	// Assemble extensions.
	var exts *wantypes.AuthenticationExtensionsClientOutputs
	if usedAppID {
		exts = &wantypes.AuthenticationExtensionsClientOutputs{AppID: true}
	}

	// Assemble authenticator data.
	// RPID (32 bytes) + User Presence (0x01, 1 byte) + Counter (4 bytes)
	authData := &bytes.Buffer{}
	if usedAppID {
		authData.Write(appIDHash[:])
	} else {
		authData.Write(rpIDHash[:])
	}
	authData.Write(authResp.RawResponse[:5]) // User Presence (1) + Counter (4)

	resp := &wantypes.CredentialAssertionResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(authCred.CredentialID),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID:      authCred.CredentialID,
			Extensions: exts,
		},
		AssertionResponse: wantypes.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: ccdJSON,
			},
			AuthenticatorData: authData.Bytes(),
			Signature:         authResp.Signature,
		},
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(resp),
		},
	}, nil
}
