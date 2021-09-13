// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webauthncli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/flynn/u2f/u2ftoken"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

// Login performs client-side, U2F-compatible Webauthn login.
// This method blocks until either device authentication is successful or the
// context is cancelled. Calling Login without a deadline or cancel condition
// may cause it block forever.
// The caller is expected to prompt the user for action before calling this
// method.
func Login(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
	switch {
	case origin == "":
		return nil, trace.BadParameter("origin required")
	case assertion == nil:
		return nil, trace.BadParameter("assertion required")
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
	if value, ok := assertion.Response.Extensions[wanlib.AppIDExtension]; ok {
		appID = fmt.Sprint(value)
		appIDHash = sha256.Sum256([]byte(appID))
	}

	// Variables below are filled by the callback on success.
	var authCred protocol.CredentialDescriptor
	var authResp *u2ftoken.AuthenticateResponse
	var usedAppID bool
	makeAuthU2F := func(cred protocol.CredentialDescriptor, req u2ftoken.AuthenticateRequest, appID bool) func(Token) error {
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
	var exts *wanlib.AuthenticationExtensionsClientOutputs
	if usedAppID {
		exts = &wanlib.AuthenticationExtensionsClientOutputs{AppID: true}
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

	resp := &wanlib.CredentialAssertionResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(authCred.CredentialID),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID:      authCred.CredentialID,
			Extensions: exts,
		},
		AssertionResponse: wanlib.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: ccdJSON,
			},
			AuthenticatorData: authData.Bytes(),
			Signature:         authResp.Signature,
		},
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
		},
	}, nil
}
