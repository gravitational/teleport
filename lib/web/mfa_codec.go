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
	"encoding/json"

	proto "github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
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
