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
	"errors"
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/touchid"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	log "github.com/sirupsen/logrus"
)

// AuthenticatorAttachment allows callers to choose a specific attachment.
type AuthenticatorAttachment int

const (
	AttachmentAuto AuthenticatorAttachment = iota
	AttachmentCrossPlatform
	AttachmentPlatform
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

// Login performs client-side, U2F-compatible, Webauthn login.
// This method blocks until either device authentication is successful or the
// context is canceled. Calling Login without a deadline or cancel condition
// may cause it to block forever.
// The informed user is used to disambiguate credentials in case of passwordless
// logins.
// It returns an MFAAuthenticateResponse and the credential user, if a resident
// credential is used.
// The caller is expected to react to LoginPrompt in order to prompt the user at
// appropriate times. Login may choose different flows depending on the type of
// authentication and connected devices.
func Login(
	ctx context.Context,
	origin string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	// origin vs RPID sanity check.
	// Doesn't necessarily means a failure, but it's likely to be one.
	switch {
	case origin == "", assertion == nil: // let downstream handle empty/nil
	case !strings.HasPrefix(origin, "https://"+assertion.Response.RelyingPartyID):
		log.Warnf(""+
			"WebAuthn: origin and RPID mismatch, "+
			"if you are having authentication problems double check your proxy address "+
			"(%q vs %q)", origin, assertion.Response.RelyingPartyID)
	}

	var attachment AuthenticatorAttachment
	var user string
	if opts != nil {
		attachment = opts.AuthenticatorAttachment
		user = opts.User
	}

	switch attachment {
	case AttachmentCrossPlatform:
		log.Debug("Cross-platform login")
		return crossPlatformLogin(ctx, origin, assertion, prompt, opts)
	case AttachmentPlatform:
		log.Debug("Platform login")
		return platformLogin(origin, user, assertion)
	default:
		log.Debug("Attempting platform login")
		resp, credentialUser, err := platformLogin(origin, user, assertion)
		if !errors.Is(err, &touchid.ErrAttemptFailed{}) {
			return resp, credentialUser, trace.Wrap(err)
		}

		log.WithError(err).Debug("Platform login failed, falling back to cross-platform")
		return crossPlatformLogin(ctx, origin, assertion, prompt, opts)
	}
}

func crossPlatformLogin(
	ctx context.Context,
	origin string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	if IsFIDO2Available() {
		log.Debug("FIDO2: Using libfido2 for assertion")
		return FIDO2Login(ctx, origin, assertion, prompt, opts)
	}

	prompt.PromptTouch()
	resp, err := U2FLogin(ctx, origin, assertion)
	return resp, "" /* credentialUser */, err
}

func platformLogin(origin, user string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, string, error) {
	resp, credentialUser, err := touchid.AttemptLogin(origin, user, assertion)
	if err != nil {
		return nil, "", err
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
		},
	}, credentialUser, nil
}

// Register performs client-side, U2F-compatible, Webauthn registration.
// This method blocks until either device authentication is successful or the
// context is canceled. Calling Register without a deadline or cancel condition
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
