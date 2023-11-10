/*
Copyright 2023 Gravitational, Inc.

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

package mfa

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// PromptConfig contains common mfa prompt config options.
type PromptConfig struct {
	mfa.PromptConfig
	// ProxyAddress is the address of the authenticating proxy. required.
	ProxyAddress string
	// WebauthnLoginFunc performs client-side Webauthn login.
	WebauthnLoginFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
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
	// WebauthnSupported indicates whether Webauthn is supported.
	WebauthnSupported bool
	// Log is a logging entry.
	Log *logrus.Entry
}

// NewPromptConfig returns a prompt config that will induce default behavior.
func NewPromptConfig(proxyAddr string, opts ...mfa.PromptOpt) *PromptConfig {
	cfg := &PromptConfig{
		ProxyAddress:      proxyAddr,
		WebauthnLoginFunc: wancli.Login,
		WebauthnSupported: wancli.HasPlatformSupport(),
		Log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentClient,
		}),
	}

	for _, opt := range opts {
		opt(&cfg.PromptConfig)
	}

	return cfg
}

// runOpts are mfa prompt run options.
type runOpts struct {
	promptTOTP     bool
	promptWebauthn bool
}

// getRunOptions gets mfa prompt run options by cross referencing the mfa challenge with prompt configuration.
func (c PromptConfig) getRunOptions(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (runOpts, error) {
	promptTOTP := chal.TOTP != nil
	promptWebauthn := chal.WebauthnChallenge != nil

	if !promptTOTP && !promptWebauthn {
		return runOpts{}, trace.BadParameter("mfa challenge is empty")
	}

	// Does the current platform support hardware MFA? Adjust accordingly.
	switch {
	case !promptTOTP && !c.WebauthnSupported:
		return runOpts{}, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
	case !c.WebauthnSupported:
		// Do not prompt for hardware devices, it won't work.
		promptWebauthn = false
	}

	// Tweak enabled/disabled methods according to opts.
	switch {
	case promptTOTP && c.PreferOTP:
		promptWebauthn = false
	case promptWebauthn && c.AuthenticatorAttachment != wancli.AttachmentAuto:
		// Prefer Webauthn if an specific attachment was requested.
		promptTOTP = false
	case promptWebauthn && !c.AllowStdinHijack:
		// Use strongest auth if hijack is not allowed.
		promptTOTP = false
	}

	return runOpts{promptTOTP, promptWebauthn}, nil
}

func (c PromptConfig) getWebauthnOrigin() string {
	if !strings.HasPrefix(c.ProxyAddress, "https://") {
		return "https://" + c.ProxyAddress
	}
	return c.ProxyAddress
}
