// Copyright 2022 Gravitational, Inc
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
	"context"

	"github.com/gravitational/teleport/api/client/proto"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	log "github.com/sirupsen/logrus"
)

// Login performs client-side, U2F-compatible, Webauthn login.
// This method blocks until either device authentication is successful or the
// context is cancelled. Calling Login without a deadline or cancel condition
// may cause it block forever.
// The informed user is used to disambiguate credentials in case of passwordless
// logins.
// It returns an MFAAuthenticateResponse and the credential user, if a resident
// credential is used.
// The caller is expected to react to LoginPrompt in order to prompt the user at
// appropriate times. Login may choose different flows depending on the type of
// authentication and connected devices.
func Login(
	ctx context.Context,
	origin string, user string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt,
) (*proto.MFAAuthenticateResponse, string, error) {
	if IsFIDO2Available() {
		log.Debug("FIDO2: Using libfido2 for assertion")
		return FIDO2Login(ctx, origin, user, assertion, prompt)
	}

	prompt.PromptTouch()
	resp, err := U2FLogin(ctx, origin, assertion)
	return resp, "" /* credentialUser */, err
}

// Register performs client-side, U2F-compatible, Webauthn registration.
// This method blocks until either device authentication is successful or the
// context is cancelled. Calling Register without a deadline or cancel condition
// may cause it block forever.
// The caller is expected to react to RegisterPrompt in order to prompt the user
// at appropriate times. Register may choose different flows depending on the
// type of authentication and connected devices.
func Register(
	ctx context.Context,
	origin string, cc *wanlib.CredentialCreation, prompt RegisterPrompt) (*proto.MFARegisterResponse, error) {
	if IsFIDO2Available() {
		log.Debug("FIDO2: Using libfido2 for credential creation")
		return FIDO2Register(ctx, origin, cc, prompt)
	}

	prompt.PromptTouch()
	return U2FRegister(ctx, origin, cc)
}
