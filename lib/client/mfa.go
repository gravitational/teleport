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

package client

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client/mfa"
)

// TODO (Joerger): remove this once the exported PromptWebauthn function is no longer used in tests.
// promptWebauthn provides indirection for tests.
var promptWebauthn func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)

// mfaPrompt implements wancli.LoginPrompt for MFA logins.
// In most cases authenticators shouldn't require PINs or additional touches for
// MFA, but the implementation exists in case we find some unusual
// authenticators out there.
type mfaPrompt struct {
	wancli.LoginPrompt
	otpCancelAndWait func()
}

func (p *mfaPrompt) PromptPIN() (string, error) {
	p.otpCancelAndWait()
	return p.LoginPrompt.PromptPIN()
}

// PromptMFAChallengeOpts groups optional settings for PromptMFAChallenge.
type PromptMFAChallengeOpts struct {
	// HintBeforePrompt is an optional hint message to print before an MFA prompt.
	// It is used to provide context about why the user is being prompted where it may
	// not be obvious.
	HintBeforePrompt string
	// PromptDevicePrefix is an optional prefix printed before "security key" or
	// "device". It is used to emphasize between different kinds of devices, like
	// registered vs new.
	PromptDevicePrefix string
	// Quiet suppresses users prompts.
	Quiet bool
	// AllowStdinHijack allows stdin hijack during MFA prompts.
	// Stdin hijack provides a better login UX, but it can be difficult to reason
	// about and is often a source of bugs.
	// Do not set this options unless you deeply understand what you are doing.
	// If false then only the strongest auth method is prompted.
	AllowStdinHijack bool
	// AuthenticatorAttachment specifies the desired authenticator attachment.
	AuthenticatorAttachment wancli.AuthenticatorAttachment
	// PreferOTP favors OTP challenges, if applicable.
	// Takes precedence over AuthenticatorAttachment settings.
	PreferOTP bool
}

// hasPlatformSupport is used to mock wancli.HasPlatformSupport for tests.
var hasPlatformSupport = wancli.HasPlatformSupport

// PromptMFAFunc matches the signature of [mfa.NewPrompt().Run].
type PromptMFAFunc func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)

// NewMFAPrompt creates a new MFA prompt from client settings.
func (tc *TeleportClient) NewMFAPrompt(opts ...func(*mfa.Prompt)) PromptMFAFunc {
	if tc.PromptMFAFunc != nil {
		return tc.PromptMFAFunc
	}

	prompt := mfa.NewPrompt(tc.WebProxyAddr)
	prompt.AuthenticatorAttachment = tc.AuthenticatorAttachment
	prompt.PreferOTP = tc.PreferOTP
	prompt.AllowStdinHijack = tc.AllowStdinHijack

	// TODO (Joerger): remove this once the exported PromptWebauthn function is no longer used in tests.
	if promptWebauthn != nil {
		prompt.WebauthnLogin = promptWebauthn
		prompt.WebauthnSupported = true
	}

	for _, opt := range opts {
		opt(prompt)
	}

	return prompt.Run
}

// PromptMFA prompts for MFA for the given challenge using the clients standard settings.
// Use [NewMFAPrompt] to create a prompt with customizable settings.
func (tc *TeleportClient) PromptMFA(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return tc.NewMFAPrompt()(ctx, chal)
}

// MFAAuthenticateChallenge is an MFA authentication challenge sent on user
// login / authentication ceremonies.
type MFAAuthenticateChallenge struct {
	// WebauthnChallenge contains a WebAuthn credential assertion used for
	// login/authentication ceremonies.
	WebauthnChallenge *wantypes.CredentialAssertion `json:"webauthn_challenge"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
}

// MakeAuthenticateChallenge converts proto to JSON format.
func MakeAuthenticateChallenge(protoChal *proto.MFAAuthenticateChallenge) *MFAAuthenticateChallenge {
	chal := &MFAAuthenticateChallenge{
		TOTPChallenge: protoChal.GetTOTP() != nil,
	}
	if protoChal.GetWebauthnChallenge() != nil {
		chal.WebauthnChallenge = wantypes.CredentialAssertionFromProto(protoChal.WebauthnChallenge)
	}
	return chal
}

type TOTPRegisterChallenge struct {
	QRCode []byte `json:"qrCode"`
}

// MFARegisterChallenge is an MFA register challenge sent on new MFA register.
type MFARegisterChallenge struct {
	// Webauthn contains webauthn challenge.
	Webauthn *wantypes.CredentialCreation `json:"webauthn"`
	// TOTP contains TOTP challenge.
	TOTP *TOTPRegisterChallenge `json:"totp"`
}

// MakeRegisterChallenge converts proto to JSON format.
func MakeRegisterChallenge(protoChal *proto.MFARegisterChallenge) *MFARegisterChallenge {
	switch protoChal.GetRequest().(type) {
	case *proto.MFARegisterChallenge_TOTP:
		return &MFARegisterChallenge{
			TOTP: &TOTPRegisterChallenge{
				QRCode: protoChal.GetTOTP().GetQRCode(),
			},
		}
	case *proto.MFARegisterChallenge_Webauthn:
		return &MFARegisterChallenge{
			Webauthn: wantypes.CredentialCreationFromProto(protoChal.GetWebauthn()),
		}
	}
	return nil
}
