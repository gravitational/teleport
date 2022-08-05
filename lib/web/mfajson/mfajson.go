/*
Copyright 2022 Gravitational, Inc.

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

package mfajson

import (
	"encoding/json"

	"github.com/gravitational/trace"

	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
)

// Decode parses a JSON-encoded MFA authentication response.
// Is either webauthn (type="n") or u2f (type="u").
func Decode(b []byte, typ string) (*authproto.MFAAuthenticateResponse, error) {
	var resp *authproto.MFAAuthenticateResponse

	switch typ {
	case defaults.WebsocketWebauthnChallenge:
		var webauthnResponse wanlib.CredentialAssertionResponse
		if err := json.Unmarshal(b, &webauthnResponse); err != nil {
			return nil, trace.Wrap(err)
		}
		resp = &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(&webauthnResponse),
			},
		}

	default:
		var u2fResponse u2f.AuthenticateChallengeResponse
		if err := json.Unmarshal(b, &u2fResponse); err != nil {
			return nil, trace.Wrap(err)
		}
		resp = &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_U2F{
				U2F: &authproto.U2FResponse{
					KeyHandle:  u2fResponse.KeyHandle,
					ClientData: u2fResponse.ClientData,
					Signature:  u2fResponse.SignatureData,
				},
			},
		}
	}

	return resp, nil
}
