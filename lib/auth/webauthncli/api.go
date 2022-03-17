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
)

// Login performs client-side, U2F-compatible, Webauthn login.
// This method blocks until either device authentication is successful or the
// context is cancelled. Calling Login without a deadline or cancel condition
// may cause it block forever.
// The caller is expected to prompt the user for action before calling this
// method.
func Login(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
	return U2FLogin(ctx, origin, assertion)
}

// Register performs client-side, U2F-compatible, Webauthn registration.
// This method blocks until either device authentication is successful or the
// context is cancelled. Calling Register without a deadline or cancel condition
// may cause it block forever.
// The caller is expected to prompt the user for action before calling this
// method.
func Register(ctx context.Context, origin string, cc *wanlib.CredentialCreation) (*proto.MFARegisterResponse, error) {
	return U2FRegister(ctx, origin, cc)
}
