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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

// ErrUsingNonRegisteredDevice is returned from Login when the user attempts to
// authenticate with a non-registered security key.
// The error message is meant to be displayed to end-users, thus it breaks the
// usual Go error conventions (capitalized sentences, punctuation).
var ErrUsingNonRegisteredDevice = errors.New("You are using a security key that is not registered with Teleport. Try a different security key.")

// AuthenticatorAttachment allows callers to choose a specific attachment.
type AuthenticatorAttachment int

const (
	AttachmentAuto AuthenticatorAttachment = iota
	AttachmentCrossPlatform
	AttachmentPlatform
)

func (a AuthenticatorAttachment) String() string {
	switch a {
	case AttachmentAuto:
		return "auto"
	case AttachmentCrossPlatform:
		return "cross-platform"
	case AttachmentPlatform:
		return "platform"
	}
	return ""
}

// CredentialInfo holds information about a WebAuthn credential, typically a
// resident public key credential.
type CredentialInfo struct {
	ID   []byte
	User UserInfo
}

// UserInfo holds information about a credential owner.
type UserInfo struct {
	// UserHandle is the WebAuthn user handle (also referred as user ID).
	UserHandle []byte
	Name       string
}

// LoginPrompt is the user interface for FIDO2Login.
//
// Prompts can have remote implementations, thus all methods may error.
type LoginPrompt interface {
	// PromptPIN prompts the user for their PIN.
	PromptPIN() (string, error)
	// PromptTouch prompts the user for a security key touch.
	// In certain situations multiple touches may be required (PIN-protected
	// devices, passwordless flows, etc).
	// Returns a TouchAcknowledger which should be called to signal to the user
	// that the touch was successfully detected.
	// Returns an error if the prompt fails to be sent to the user.
	PromptTouch() (TouchAcknowledger, error)
	// PromptCredential prompts the user to choose a credential, in case multiple
	// credentials are available.
	// Callers are free to modify the slice, such as by sorting the credentials,
	// but must return one of the pointers contained within.
	PromptCredential(creds []*CredentialInfo) (*CredentialInfo, error)
}

// TouchAcknowledger is a function type which should be called to signal to the
// user that a security key touch was successfully detected.
// May return an error if the acknowledgement fails to be sent to the user.
type TouchAcknowledger func() error

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
	ctx, span := tracing.NewTracer("mfa").Start(
		ctx,
		"webauthncli/Login",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

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

	if webauthnwin.IsAvailable() {
		log.Debug("WebAuthnWin: Using windows webauthn for credential assertion")
		return webauthnwin.Login(ctx, origin, assertion, &webauthnwin.LoginOpts{
			AuthenticatorAttachment: webauthnwin.AuthenticatorAttachment(attachment),
		})
	}

	switch attachment {
	case AttachmentCrossPlatform:
		log.Debug("Cross-platform login")
		return crossPlatformLogin(ctx, origin, assertion, prompt, opts)
	case AttachmentPlatform:
		log.Debug("Platform login")
		return platformLogin(origin, user, assertion, prompt)
	default:
		log.Debug("Attempting platform login")
		resp, credentialUser, err := platformLogin(origin, user, assertion, prompt)
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
	if isLibfido2Enabled() {
		log.Debug("FIDO2: Using libfido2 for assertion")
		return FIDO2Login(ctx, origin, assertion, prompt, opts)
	}

	ackTouch, err := prompt.PromptTouch()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	resp, err := U2FLogin(ctx, origin, assertion)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if err := ackTouch(); err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp, "" /* credentialUser */, err
}

func platformLogin(origin, user string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt) (*proto.MFAAuthenticateResponse, string, error) {
	resp, credentialUser, err := touchid.AttemptLogin(origin, user, assertion, ToTouchIDCredentialPicker(prompt))
	if err != nil {
		return nil, "", err
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
		},
	}, credentialUser, nil
}

// RegisterPrompt is the user interface for FIDO2Register.
//
// Prompts can have remote implementations, thus all methods may error.
type RegisterPrompt interface {
	// PromptPIN prompts the user for their PIN.
	PromptPIN() (string, error)
	// PromptTouch prompts the user for a security key touch.
	// In certain situations multiple touches may be required (PIN-protected
	// devices, passwordless flows, etc).
	// Returns a TouchAcknowledger which should be called to signal to the user
	// that the touch was successfully detected.
	// Returns an error if the prompt fails to be sent to the user.
	PromptTouch() (TouchAcknowledger, error)
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
	if webauthnwin.IsAvailable() {
		log.Debug("WebAuthnWin: Using windows webauthn for credential creation")
		return webauthnwin.Register(ctx, origin, cc)
	}

	if isLibfido2Enabled() {
		log.Debug("FIDO2: Using libfido2 for credential creation")
		return FIDO2Register(ctx, origin, cc, prompt)
	}

	ackTouch, err := prompt.PromptTouch()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := U2FRegister(ctx, origin, cc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, trace.Wrap(ackTouch())
}

// HasPlatformSupport returns true if the platform supports client-side
// WebAuthn-compatible logins.
func HasPlatformSupport() bool {
	return IsFIDO2Available() || touchid.IsAvailable() || isU2FAvailable()
}

// IsFIDO2Available returns true if FIDO2 is implemented either via native
// libfido2 library or Windows WebAuthn API.
func IsFIDO2Available() bool {
	return isLibfido2Enabled() || webauthnwin.IsAvailable()
}
