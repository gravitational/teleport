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

package mfajson

import (
	"encoding/json"

	"github.com/gravitational/trace"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
)

// TODO(Joerger): DELETE IN v19.0.0 and use client.MFAChallengeResponse instead.
// Before v17, the WebUI sends a flattened webauthn response instead of a full
// MFA challenge response. Newer WebUI versions v17+ will send both for
// backwards compatibility.
type challengeResponse struct {
	client.MFAChallengeResponse
	*wantypes.CredentialAssertionResponse
}

// Decode parses a JSON-encoded MFA authentication response.
// Only webauthn (type="n") is currently supported.
func Decode(b []byte, typ string) (*authproto.MFAAuthenticateResponse, error) {
	var resp challengeResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, trace.Wrap(err)
	}

	// Move flattened webauthn response into resp.
	resp.MFAChallengeResponse.WebauthnAssertionResponse = resp.CredentialAssertionResponse

	protoResp, err := resp.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if protoResp == nil {
		return nil, trace.BadParameter("invalid MFA response from web")
	}

	return protoResp, trace.Wrap(err)
}
