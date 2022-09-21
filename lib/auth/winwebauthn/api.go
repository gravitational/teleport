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

// Package winwebauthn is wrapper around Windows webauthn API.
// It loads system webauthn.dll and uses it's method.
// It supports API versions 1-4.
// API definition: https://github.com/microsoft/webauthn/blob/master/webauthn.h
package winwebauthn

import (
	"context"
	"errors"
	"io"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/gravitational/teleport/api/client/proto"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/trace"
)

var (
	ErrNotAllowed = errors.New("NotAllowed error")
)

// LoginOpts groups non-mandatory options for Login.
type LoginOpts struct {
	// User is the desired credential username for login.
	// If empty, Login may either choose a credential or prompt the user for input
	// (via LoginPrompt).
	User string
	// AuthenticatorAttachment specifies the desired authenticator attachment.
	AuthenticatorAttachment AuthenticatorAttachment
}

type AuthenticatorAttachment int

const (
	AttachmentAuto AuthenticatorAttachment = iota
	AttachmentCrossPlatform
	AttachmentPlatform
)

// Login implements Login for WindowsHello, CTAP1 and CTAP2 devices.
// The informed user is used to disambiguate credentials in case of passwordless
// logins.
// It returns an MFAAuthenticateResponse and the credential user, if a resident
// credential is used.
// Most callers should call Login directly, as it is correctly guarded by
// IsAvailable.
func Login(ctx context.Context,
	origin string, assertion *wanlib.CredentialAssertion,
	opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	return login(ctx, origin, assertion, opts)
}

// Register implements Register for WindowsHello, CTAP1 and CTAP2 devices.
// Most callers should call Register directly, as it is correctly guarded by
// IsAvailable.
func Register(
	ctx context.Context,
	origin string, cc *wanlib.CredentialCreation,
) (*proto.MFARegisterResponse, error) {
	return register(ctx, origin, cc)
}

// CheckSupport is the result from a Windows webauthn support check.
type CheckSupportResult struct {
	HasCompileSupport bool
	IsAvailable       bool
	HasPlatformUV     bool
	APIVersion        int
}

// IsAvailable returns true if Touch ID is available in the system.
// Typically, a series of checks is performed in an attempt to avoid false
// positives.
// See CheckSupport.
func IsAvailable() bool {
	return isAvailable()
}

// TODO(tobiaszheller): I already have RunDiagnostics which uses user itneractions,consider rename.
func CheckSupport() (*CheckSupportResult, error) {
	return checkSupport()
}

type RunDiagnosticsResult struct {
	Available                           bool
	RegisterSuccessful, LoginSuccessful bool
}

// RunDiagnostics runs a few diagnostic commands and returns the result.
// User interaction is required.
func RunDiagnostics(ctx context.Context, promptOut io.Writer) (*RunDiagnosticsResult, error) {
	res := &RunDiagnosticsResult{}
	if !isAvailable() {
		return res, nil
	}
	res.Available = true

	// Attempt registration.
	const origin = "localhost"
	cc := &wanlib.CredentialCreation{
		Response: protocol.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: protocol.RelyingPartyEntity{
				ID: "localhost",
				CredentialEntity: protocol.CredentialEntity{
					Name: "test RP",
				},
			},
			User: protocol.UserEntity{
				CredentialEntity: protocol.CredentialEntity{
					Name: "test",
				},
				ID:          []byte("test"),
				DisplayName: "test",
			},
			Parameters: []protocol.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgRS256,
				},
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}
	ccr, err := Register(ctx, origin, cc)
	if err != nil {
		return res, trace.Wrap(err)
	}
	res.RegisterSuccessful = true

	// Attempt login.
	assertion := &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: cc.Response.RelyingParty.ID,
			AllowedCredentials: []protocol.CredentialDescriptor{
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: ccr.GetWebauthn().GetRawId(),
				},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}
	if _, _, err := Login(ctx, origin, assertion, &LoginOpts{}); err != nil {
		return res, trace.Wrap(err)
	}
	res.LoginSuccessful = true

	return res, nil
}
