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

package web

import (
	"bytes"
	"encoding/json"

	proto "github.com/gogo/protobuf/proto"
	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/trace"
)

// mfaCodec converts MFA challenges/responses between their native types and a format
// suitable for being sent over a network connection.
type mfaCodec interface {
	// encode converts an MFA challenge to wire format
	encode(chal *auth.MFAAuthenticateChallenge, envelopeType string) ([]byte, error)

	// decode parses an MFA authentication response
	decode(bytes []byte, envelopeType string) (*authproto.MFAAuthenticateResponse, error)
}

// protobufMFACodec converts MFA challenges and responses to the protobuf
// format used by SSH web sessions
type protobufMFACodec struct{}

func (protobufMFACodec) encode(chal *auth.MFAAuthenticateChallenge, envelopeType string) ([]byte, error) {
	chalEnc, err := json.Marshal(chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    envelopeType,
		Payload: string(chalEnc),
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return envelopeBytes, nil
}

func (protobufMFACodec) decode(bytes []byte, envelopeType string) (*authproto.MFAAuthenticateResponse, error) {
	envelope := &Envelope{}
	if err := proto.Unmarshal(bytes, envelope); err != nil {
		return nil, trace.Wrap(err)
	}

	var mfaResponse *authproto.MFAAuthenticateResponse

	switch envelopeType {
	case defaults.WebsocketWebauthnChallenge:
		var webauthnResponse wanlib.CredentialAssertionResponse
		if err := json.Unmarshal([]byte(envelope.Payload), &webauthnResponse); err != nil {
			return nil, trace.Wrap(err)
		}
		mfaResponse = &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(&webauthnResponse),
			},
		}

	default:
		var u2fResponse u2f.AuthenticateChallengeResponse
		if err := json.Unmarshal([]byte(envelope.Payload), &u2fResponse); err != nil {
			return nil, trace.Wrap(err)
		}
		mfaResponse = &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_U2F{
				U2F: &authproto.U2FResponse{
					KeyHandle:  u2fResponse.KeyHandle,
					ClientData: u2fResponse.ClientData,
					Signature:  u2fResponse.SignatureData,
				},
			},
		}
	}

	return mfaResponse, nil
}

// tdpMFACodec converts MFA challenges and responses to Teleport Desktop
// Protocol (TDP) messages used by Desktop Access web sessions
type tdpMFACodec struct{}

func (tdpMFACodec) encode(chal *auth.MFAAuthenticateChallenge, envelopeType string) ([]byte, error) {
	var mfaType byte
	switch envelopeType {
	case defaults.WebsocketWebauthnChallenge:
		mfaType = tdp.WebsocketWebauthnChallengeByte
	case defaults.WebsocketU2FChallenge:
		mfaType = tdp.WebsocketU2FChallengeByte
	default:
		return nil, trace.BadParameter("received envelope type %v, expected either %v (WebAuthn) or %v (U2F)",
			envelopeType, defaults.WebsocketWebauthnChallenge, defaults.WebsocketU2FChallenge)
	}

	mfaChal := tdp.MFAJson{
		MfaType:                  mfaType,
		MFAAuthenticateChallenge: chal,
	}
	return mfaChal.Encode()
}

func (tdpMFACodec) decode(buf []byte, envelopeType string) (*authproto.MFAAuthenticateResponse, error) {
	mfaJson, err := tdp.DecodeMFAJson(bytes.NewReader(buf))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return mfaJson.MFAAuthenticateResponse, nil
}
