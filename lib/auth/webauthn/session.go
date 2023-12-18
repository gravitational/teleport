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

package webauthn

import (
	"encoding/base64"

	"github.com/go-webauthn/webauthn/protocol"
	wan "github.com/go-webauthn/webauthn/webauthn"
	"github.com/gravitational/trace"

	wanpb "github.com/gravitational/teleport/api/types/webauthn"
)

// scopeLogin identifies session data stored for login.
// It is used as the scope for global session data and as the sessionID for
// per-user session data.
// Only one in-flight login is supported for MFA / per-user session data.
const scopeLogin = "login"

// scopeSession is used as the per-user sessionID for registrations.
// Only one in-flight registration is supported per-user, baring registrations
// that use in-memory storage.
const scopeSession = "registration"

func sessionToPB(sd *wan.SessionData) (*wanpb.SessionData, error) {
	rawChallenge, err := base64.RawURLEncoding.DecodeString(sd.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(codingllama): Record extensions in stored session data.
	return &wanpb.SessionData{
		Challenge:        rawChallenge,
		UserId:           sd.UserID,
		AllowCredentials: sd.AllowedCredentialIDs,
		UserVerification: string(sd.UserVerification),
	}, nil
}

func sessionFromPB(sd *wanpb.SessionData) *wan.SessionData {
	// TODO(codingllama): Record extensions in stored session data.
	return &wan.SessionData{
		Challenge:            base64.RawURLEncoding.EncodeToString(sd.Challenge),
		UserID:               sd.UserId,
		AllowedCredentialIDs: sd.AllowCredentials,
		UserVerification:     protocol.UserVerificationRequirement(sd.UserVerification),
	}
}
