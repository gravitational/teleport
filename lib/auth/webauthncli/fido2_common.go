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

package webauthncli

import (
	"context"
	"io"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// FIDO2PollInterval is the poll interval used to check for new FIDO2 devices.
var FIDO2PollInterval = 200 * time.Millisecond

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
	origin string, assertion *wantypes.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	return fido2Login(ctx, origin, assertion, prompt, opts)
}

// FIDO2Register implements Register for CTAP1 and CTAP2 devices.
// It must be called with a context with timeout, otherwise it can run
// indefinitely.
// Most callers should call Register directly, as it is correctly guarded by
// IsFIDO2Available.
func FIDO2Register(
	ctx context.Context,
	origin string, cc *wantypes.CredentialCreation, prompt RegisterPrompt,
) (*proto.MFARegisterResponse, error) {
	return fido2Register(ctx, origin, cc, prompt)
}

type FIDO2DiagResult struct {
	Available                           bool
	RegisterSuccessful, LoginSuccessful bool
}

// FIDO2Diag runs a few diagnostic commands and returns the result.
// User interaction is required.
func FIDO2Diag(ctx context.Context, promptOut io.Writer) (*FIDO2DiagResult, error) {
	res := &FIDO2DiagResult{}
	if !isLibfido2Enabled() {
		return res, nil
	}
	res.Available = true

	// Attempt registration.
	const origin = "localhost"
	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "localhost",
				},
				ID: "localhost",
			},
			User: wantypes.UserEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "test",
				},
				DisplayName: "test",
				ID:          []byte("test"),
			},
			Parameters: []wantypes.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgES256,
				},
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}
	prompt := NewDefaultPrompt(ctx, promptOut)
	ccr, err := FIDO2Register(ctx, origin, cc, prompt)
	if err != nil {
		return res, trace.Wrap(err)
	}
	res.RegisterSuccessful = true

	// Attempt login.
	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: cc.Response.RelyingParty.ID,
			AllowedCredentials: []wantypes.CredentialDescriptor{
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: ccr.GetWebauthn().GetRawId(),
				},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}
	prompt = NewDefaultPrompt(ctx, promptOut) // Avoid reusing prompts
	if _, _, err := FIDO2Login(ctx, origin, assertion, prompt, nil /* opts */); err != nil {
		return res, trace.Wrap(err)
	}
	res.LoginSuccessful = true

	return res, nil
}
