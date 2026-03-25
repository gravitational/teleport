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
	"errors"
	"strings"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	wanwin "github.com/gravitational/teleport/lib/auth/webauthnwin"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	log     = logutils.NewPackageLogger(teleport.ComponentKey, "WebAuthn")
	fidoLog = logutils.NewPackageLogger(teleport.ComponentKey, "FIDO2")
)

// ErrUsingNonRegisteredDevice is returned from Login when the user attempts to
// authenticate with a non-registered security key.
// The error message is meant to be displayed to end-users, thus it breaks the
// usual Go error conventions (capitalized sentences, punctuation).
var ErrUsingNonRegisteredDevice = errors.New("you are using a security key that is not registered with Teleport - try a different security key")

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

// Login performs client-side, Webauthn login.
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
	origin string, assertion *wantypes.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
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
		log.WarnContext(ctx, "origin and RPID mismatch, if you are having authentication problems double check your proxy address",
			"origin", origin,
			"rpid", assertion.Response.RelyingPartyID,
		)
	}

	var attachment AuthenticatorAttachment
	var user string
	if opts != nil {
		attachment = opts.AuthenticatorAttachment
		user = opts.User
	}

	if wanwin.IsAvailable() {
		log.DebugContext(ctx, "Using windows webauthn for credential assertion")
		return wanwin.Login(ctx, origin, assertion, &wanwin.LoginOpts{
			AuthenticatorAttachment: wanwin.AuthenticatorAttachment(attachment),
		})
	}

	switch attachment {
	case AttachmentCrossPlatform:
		log.DebugContext(ctx, "Cross-platform login")
		return crossPlatformLogin(ctx, origin, assertion, prompt, opts)
	case AttachmentPlatform:
		log.DebugContext(ctx, "Platform login")
		return platformLogin(origin, user, assertion, prompt)
	default:
		log.DebugContext(ctx, "Attempting platform login")
		resp, credentialUser, err := platformLogin(origin, user, assertion, prompt)
		if !errors.Is(err, &touchid.ErrAttemptFailed{}) {
			return resp, credentialUser, trace.Wrap(err)
		}

		log.DebugContext(ctx, "Platform login failed, falling back to cross-platform", "error", err)
		return crossPlatformLogin(ctx, origin, assertion, prompt, opts)
	}
}

func crossPlatformLogin(
	ctx context.Context,
	origin string, assertion *wantypes.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	fidoLog.DebugContext(ctx, "Using libfido2 for assertion")
	resp, user, err := FIDO2Login(ctx, origin, assertion, prompt, opts)
	return resp, user, trace.Wrap(err)
}

func platformLogin(origin, user string, assertion *wantypes.CredentialAssertion, prompt LoginPrompt) (*proto.MFAAuthenticateResponse, string, error) {
	resp, credentialUser, err := touchid.AttemptLogin(origin, user, assertion, ToTouchIDCredentialPicker(prompt))
	if err != nil {
		return nil, "", err
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(resp),
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

// Register performs client-side, Webauthn registration.
// This method blocks until either device authentication is successful or the
// context is canceled. Calling Register without a deadline or cancel condition
// may cause it block forever.
// The caller is expected to react to RegisterPrompt in order to prompt the user
// at appropriate times. Register may choose different flows depending on the
// type of authentication and connected devices.
func Register(
	ctx context.Context,
	origin string, cc *wantypes.CredentialCreation, prompt RegisterPrompt) (*proto.MFARegisterResponse, error) {
	if wanwin.IsAvailable() {
		log.DebugContext(ctx, "Using windows webauthn for credential creation")
		return wanwin.Register(ctx, origin, cc)
	}

	fidoLog.DebugContext(ctx, "Using libfido2 for credential creation")
	resp, err := FIDO2Register(ctx, origin, cc, prompt)
	return resp, trace.Wrap(err)
}

// HasPlatformSupport returns true if the platform supports client-side
// WebAuthn-compatible logins.
func HasPlatformSupport() bool {
	return IsFIDO2Available() || touchid.IsAvailable()
}

// IsFIDO2Available returns true if FIDO2 is implemented either via native
// libfido2 library or Windows WebAuthn API.
func IsFIDO2Available() bool {
	return isLibfido2Enabled() || wanwin.IsAvailable()
}
