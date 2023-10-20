/*
Copyright 2021 Gravitational, Inc.

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
