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

package winwebauthn

import (
	"context"
	"io"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/gravitational/teleport/api/client/proto"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/trace"
)

// Login implements Login for WindowsHello, CTAP1 and CTAP2 devices.
// The informed user is used to disambiguate credentials in case of passwordless
// logins.
// It returns an MFAAuthenticateResponse and the credential user, if a resident
// credential is used.
// Most callers should call Login directly, as it is correctly guarded by
// IsAvailable.
func Login(
	ctx context.Context,
	origin string, assertion *wanlib.CredentialAssertion,
) (*proto.MFAAuthenticateResponse, string, error) {
	return login(ctx, origin, assertion)
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

// DiagResult is the result from a Windows webauthn self diagnostics check.
type DiagResult struct {
	HasCompileSupport bool
	HasSignature      bool
	IsAvailable       bool
	HasPlatformUV     bool
	APIVersion        int
}

// IsAvailable returns true if Touch ID is available in the system.
// Typically, a series of checks is performed in an attempt to avoid false
// positives.
// See Diag.
func IsAvailable() bool {
	return isAvailable()
}

// TODO(tobiaszheller): I already have RunDiagnostics which uses user itneractions,consider rename.
func Diag() (*DiagResult, error) {
	return diag()
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
	if _, _, err := Login(ctx, origin, assertion); err != nil {
		return res, trace.Wrap(err)
	}
	res.LoginSuccessful = true

	return res, nil
}
