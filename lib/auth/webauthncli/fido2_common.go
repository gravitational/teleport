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
	"time"

	"github.com/gravitational/teleport/api/client/proto"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

// FIDO2PollInterval is the poll interval used to check for new FIDO2 devices.
var FIDO2PollInterval = 200 * time.Millisecond

// LoginPrompt is the user interface for FIDO2Login.
type LoginPrompt interface {
	// PromptPIN prompts the user for their PIN.
	PromptPIN() (string, error)
	// PromptTouch prompts the user for a security key touch.
	// In certain situations multiple touches may be required (PIN-protected
	// devices, passwordless flows, etc).
	PromptTouch()
}

// FIDO2Login implements Login for CTAP1 and CTAP2 devices.
// It must be called with a context with timeout, otherwise it can run
// indefinitely.
// The informed user is used to disambiguate credentials in case of passwordless
// logins.
// It returns an MFAAuthenticateResponse and the credential user, if a resident
// credential is used.
// Most callers should call Login directly, as it is correctly guarded by
// IsFIDO2Available.
func FIDO2Login(
	ctx context.Context,
	origin, user string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt,
) (*proto.MFAAuthenticateResponse, string, error) {
	return fido2Login(ctx, origin, user, assertion, prompt)
}

// RegisterPrompt is the user interface for FIDO2Register.
type RegisterPrompt interface {
	// PromptPIN prompts the user for their PIN.
	PromptPIN() (string, error)
	// PromptTouch prompts the user for a security key touch.
	// In certain situations multiple touches may be required (eg, PIN-protected
	// devices)
	PromptTouch()
}

// FIDO2Register implements Register for CTAP1 and CTAP2 devices.
// It must be called with a context with timeout, otherwise it can run
// indefinitely.
// Most callers should call Register directly, as it is correctly guarded by
// IsFIDO2Available.
func FIDO2Register(
	ctx context.Context,
	origin string, cc *wanlib.CredentialCreation, prompt RegisterPrompt,
) (*proto.MFARegisterResponse, error) {
	return fido2Register(ctx, origin, cc, prompt)
}
