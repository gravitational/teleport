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
	"fmt"
	"os"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentClient,
})

// Prompt is an MFA prompt.
type Prompt struct {
	// PromptMFA is an interchangeable MFA prompt handler using the common config options below.
	PromptMFA
	// PromptConfig contains mfa prompt config options.
	PromptConfig
}

// PromptConfig contains mfa prompt config options.
type PromptConfig struct {
	// ProxyAddress is the address of the authenticating proxy. required.
	ProxyAddress string
	// PromptReason is an optional message to share with the user before an MFA Prompt.
	// It is intended to provide context about why the user is being prompted where it may
	// not be obvious, such as for admin actions or per-session MFA.
	PromptReason string
	// DeviceType is an optional device description to emphasize during the prompt.
	DeviceType DeviceDescriptor
	// WebauthnLoginFunc performs client-side Webauthn login.
	WebauthnLoginFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
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
	// WebauthnSupported indicates whether Webauthn is supported.
	WebauthnSupported bool
}

type PromptMFA interface {
	PromptTOTP(ctx context.Context, chal *proto.MFAAuthenticateChallenge, cfg PromptConfig) (*proto.MFAAuthenticateResponse, error)
	PromptWebauthn(ctx context.Context, chal *proto.MFAAuthenticateChallenge, cfg PromptConfig) (*proto.MFAAuthenticateResponse, error)
	PromptWebauthnAndTOTP(ctx context.Context, chal *proto.MFAAuthenticateChallenge, cfg PromptConfig) (*proto.MFAAuthenticateResponse, error)
}

// DeviceDescriptor is a descriptor for a device, such as "registered".
type DeviceDescriptor string

// DeviceDescriptorRegistered is a registered device.
const DeviceDescriptorRegistered = "registered"

// PromptOpt applies configuration options to a prompt.
type PromptOpt func(*Prompt)

// WithQuiet sets the prompt's Quiet field.
func WithQuiet() PromptOpt {
	return func(p *Prompt) {
		p.Quiet = true
	}
}

// WithPromptReason sets the prompt's PromptReason field.
func WithPromptReason(hint string) PromptOpt {
	return func(p *Prompt) {
		p.PromptReason = hint
	}
}

// WithPromptReasonAdminAction sets the prompt's PromptReason field to a standard admin action message.
func WithPromptReasonAdminAction() PromptOpt {
	const adminMFAPromptReason = "MFA is required for admin-level API request."
	return WithPromptReason(adminMFAPromptReason)
}

// WithPromptReasonSessionMFA sets the prompt's PromptReason field to a standard session mfa message.
func WithPromptReasonSessionMFA(serviceType, serviceName string) PromptOpt {
	return WithPromptReason(fmt.Sprintf("MFA is required to access %s %q", serviceType, serviceName))
}

// WithPromptDeviceType sets the prompt's DeviceType field.
func WithPromptDeviceType(deviceType DeviceDescriptor) PromptOpt {
	return func(p *Prompt) {
		p.DeviceType = deviceType
	}
}

// NewPrompt creates a new prompt with default behavior.
// If you want to customize [Prompt], for example for testing purposes, you may
// create or configure an instance directly, without calling this method.
func NewPrompt(proxyAddr string) *Prompt {
	return &Prompt{
		PromptMFA: NewCLIPrompt(os.Stderr),
		PromptConfig: PromptConfig{
			WebauthnLoginFunc: wancli.Login,
			ProxyAddress:      proxyAddr,
			WebauthnSupported: wancli.HasPlatformSupport(),
		},
	}
}

// Run prompts the user to complete MFA authentication challenges according to the prompt's configuration.
func (p *Prompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	ctx, span := tracing.NewTracer("MFACeremony").Start(
		ctx,
		"Run",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	hasTOTP := chal.TOTP != nil
	hasWebauthn := chal.WebauthnChallenge != nil

	// Does the current platform support hardware MFA? Adjust accordingly.
	if !p.WebauthnSupported {
		// Do not prompt for hardware devices, it won't work.
		hasWebauthn = false
		if !hasTOTP {
			return nil, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
		}
	}

	// Tweak enabled/disabled methods according to opts.
	switch {
	case hasTOTP && p.PreferOTP:
		return p.PromptTOTP(ctx, chal, p.PromptConfig)
	case hasWebauthn && p.AuthenticatorAttachment != wancli.AttachmentAuto:
		// Prefer Webauthn if an specific attachment was requested.
		return p.PromptWebauthn(ctx, chal, p.PromptConfig)
	case hasWebauthn && !p.AllowStdinHijack:
		// Use strongest auth if hijack is not allowed.
		return p.PromptWebauthn(ctx, chal, p.PromptConfig)
	}

	switch {
	case hasTOTP && hasWebauthn:
		switch {
		case p.PreferOTP:
			return p.PromptTOTP(ctx, chal, p.PromptConfig)
		case p.AuthenticatorAttachment != wancli.AttachmentAuto:
			// Prefer Webauthn if an specific attachment was requested.
			return p.PromptWebauthn(ctx, chal, p.PromptConfig)
		case !p.AllowStdinHijack:
			return p.PromptWebauthn(ctx, chal, p.PromptConfig)
		}
		return p.PromptWebauthnAndTOTP(ctx, chal, p.PromptConfig)
	case hasTOTP:
		return p.PromptTOTP(ctx, chal, p.PromptConfig)
	case hasWebauthn:
		return p.PromptWebauthn(ctx, chal, p.PromptConfig)
	}

	// No challenge present, return an empty response.
	return &proto.MFAAuthenticateResponse{}, nil
}

func (c PromptConfig) promptDevicePrefix() string {
	if c.DeviceType != "" {
		return fmt.Sprintf("*%s* ", c.DeviceType)
	}
	return ""
}

// WebauthnLogin performs client-side Webauthn login from prompt settings.
func (c PromptConfig) WebauthnLogin(ctx context.Context, chal *proto.MFAAuthenticateChallenge, loginPrompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
	origin := c.ProxyAddress
	if !strings.HasPrefix(origin, "https://") {
		origin = "https://" + origin
	}

	log.Debugf("WebAuthn: prompting devices with origin %q", origin)

	opts := &wancli.LoginOpts{AuthenticatorAttachment: c.AuthenticatorAttachment}
	resp, _, err := c.WebauthnLoginFunc(ctx, origin, wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge), loginPrompt, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}
