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

package web

import (
	"bytes"
	"encoding/json"

	proto "github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/web/mfajson"
	"github.com/gravitational/teleport/lib/web/terminal"
)

// protobufMFACodec converts MFA challenges and responses to the protobuf
// format used by SSH web sessions
type protobufMFACodec struct{}

func (protobufMFACodec) Encode(chal *client.MFAAuthenticateChallenge, envelopeType string) ([]byte, error) {
	jsonBytes, err := json.Marshal(chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    envelopeType,
		Payload: string(jsonBytes),
	}
	protoBytes, err := proto.Marshal(envelope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protoBytes, nil
}

func (protobufMFACodec) DecodeResponse(bytes []byte, envelopeType string) (*authproto.MFAAuthenticateResponse, error) {
	return mfajson.Decode(bytes, envelopeType)
}

func (protobufMFACodec) DecodeChallenge(bytes []byte, envelopeType string) (*authproto.MFAAuthenticateChallenge, error) {
	var challenge client.MFAAuthenticateChallenge
	if err := json.Unmarshal(bytes, &challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	return &authproto.MFAAuthenticateChallenge{
		WebauthnChallenge: wantypes.CredentialAssertionToProto(challenge.WebauthnChallenge),
	}, nil
}

// tdpMFACodec converts MFA challenges and responses to Teleport Desktop
// Protocol (TDP) messages used by Desktop Access web sessions
type tdpMFACodec struct{}

func (tdpMFACodec) Encode(chal *client.MFAAuthenticateChallenge, envelopeType string) ([]byte, error) {
	switch envelopeType {
	case defaults.WebsocketMFAChallenge:
	default:
		return nil, trace.BadParameter(
			"received envelope type %v, expected %v (MFAChallenge)", envelopeType, defaults.WebsocketMFAChallenge)
	}

	tdpMsg := tdp.MFA{
		Type:                     envelopeType[0],
		MFAAuthenticateChallenge: chal,
	}
	return tdpMsg.Encode()
}

func (tdpMFACodec) DecodeResponse(buf []byte, envelopeType string) (*authproto.MFAAuthenticateResponse, error) {
	if len(buf) == 0 {
		return nil, trace.BadParameter("empty MFA message received")
	}
	if tdp.MessageType(buf[0]) != tdp.TypeMFA {
		return nil, trace.BadParameter("decodeResponse: expected MFA message type %v, got %v", tdp.TypeMFA, buf[0])
	}
	msg, err := tdp.DecodeMFA(bytes.NewReader(buf[1:]))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return msg.MFAAuthenticateResponse, nil
}

func (tdpMFACodec) DecodeChallenge(buf []byte, envelopeType string) (*authproto.MFAAuthenticateChallenge, error) {
	if len(buf) == 0 {
		return nil, trace.BadParameter("empty MFA message received")
	}
	if tdp.MessageType(buf[0]) != tdp.TypeMFA {
		return nil, trace.BadParameter("decodeChallenge: expected MFA message type %v, got %v", tdp.TypeMFA, buf[0])
	}
	msg, err := tdp.DecodeMFAChallenge(bytes.NewReader(buf[1:]))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authproto.MFAAuthenticateChallenge{
		WebauthnChallenge: wantypes.CredentialAssertionToProto(msg.WebauthnChallenge),
	}, nil
}
