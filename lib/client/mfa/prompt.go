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
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

// AdminMFAHintBeforePrompt is a hint used for MFA prompts for admin-level API requests.
const AdminMFAHintBeforePrompt = "MFA is required for admin-level API request."

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentClient,
})

// Prompt is an MFA prompt.
type Prompt struct {
	// WebauthnLogin performs client-side Webauthn login.
	WebauthnLogin func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
	// ProxyAddress is the address of the authenticating proxy. required.
	ProxyAddress string
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
	// WebauthnSupported indicates whether Webauthn is supported.
	WebauthnSupported bool
}

// PromptOpt applies configuration options to a prompt.
type PromptOpt func(*Prompt)

// WithQuiet sets the prompt's Quiet field.
func WithQuiet() PromptOpt {
	return func(p *Prompt) {
		p.Quiet = true
	}
}

// WithHintBeforePrompt sets the prompt's HintBeforePrompt field.
func WithHintBeforePrompt(hint string) PromptOpt {
	return func(p *Prompt) {
		p.HintBeforePrompt = hint
	}
}

// WithPromptDevicePrefix sets the prompt's PromptDevicePrefix field.
func WithPromptDevicePrefix(prefix string) PromptOpt {
	return func(p *Prompt) {
		p.PromptDevicePrefix = prefix
	}
}

// NewPrompt creates a new prompt with standard behavior.
// If you want to customize [Prompt], for example for testing purposes, you may
// create or configure an instance directly, without calling this method.
func NewPrompt(proxyAddr string) *Prompt {
	return &Prompt{
		WebauthnLogin:     wancli.Login,
		ProxyAddress:      proxyAddr,
		WebauthnSupported: wancli.HasPlatformSupport(),
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

	// Is there a challenge present?
	if chal.TOTP == nil && chal.WebauthnChallenge == nil {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	writer := os.Stderr
	if p.HintBeforePrompt != "" {
		fmt.Fprintln(writer, p.HintBeforePrompt)
	}

	promptDevicePrefix := p.PromptDevicePrefix
	if promptDevicePrefix != "" {
		promptDevicePrefix += " "
	}

	quiet := p.Quiet

	hasTOTP := chal.TOTP != nil
	hasWebauthn := chal.WebauthnChallenge != nil

	// Does the current platform support hardware MFA? Adjust accordingly.
	switch {
	case !hasTOTP && !p.WebauthnSupported:
		return nil, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
	case !p.WebauthnSupported:
		// Do not prompt for hardware devices, it won't work.
		hasWebauthn = false
	}

	// Tweak enabled/disabled methods according to opts.
	switch {
	case hasTOTP && p.PreferOTP:
		hasWebauthn = false
	case hasWebauthn && p.AuthenticatorAttachment != wancli.AttachmentAuto:
		// Prefer Webauthn if an specific attachment was requested.
		hasTOTP = false
	case hasWebauthn && !p.AllowStdinHijack:
		// Use strongest auth if hijack is not allowed.
		hasTOTP = false
	}

	var numGoroutines int
	if hasTOTP && hasWebauthn {
		numGoroutines = 2
	} else {
		numGoroutines = 1
	}

	type response struct {
		kind string
		resp *proto.MFAAuthenticateResponse
		err  error
	}
	respC := make(chan response, numGoroutines)

	// Use ctx and wg to clean up after ourselves.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	cancelAndWait := func() {
		cancel()
		wg.Wait()
	}

	// Use variables below to cancel OTP reads and make sure the goroutine exited.
	otpWait := &sync.WaitGroup{}
	otpCtx, otpCancel := context.WithCancel(ctx)
	defer otpCancel()

	// Fire TOTP goroutine.
	if hasTOTP {
		otpWait.Add(1)
		wg.Add(1)
		go func() {
			defer otpWait.Done()
			defer wg.Done()
			const kind = "TOTP"

			// Let Webauthn take the prompt, it knows better if it's necessary.
			var msg string
			if !quiet && !hasWebauthn {
				msg = fmt.Sprintf("Enter an OTP code from a %sdevice", promptDevicePrefix)
			}

			otp, err := prompt.Password(otpCtx, writer, prompt.Stdin(), msg)
			if err != nil {
				respC <- response{kind: kind, err: err}
				return
			}
			respC <- response{
				kind: kind,
				resp: &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{Code: otp},
					},
				},
			}
		}()
	}

	// Fire Webauthn goroutine.
	if hasWebauthn {
		origin := p.ProxyAddress
		if !strings.HasPrefix(origin, "https://") {
			origin = "https://" + origin
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Debugf("WebAuthn: prompting devices with origin %q", origin)

			prompt := wancli.NewDefaultPrompt(ctx, writer)
			prompt.SecondTouchMessage = fmt.Sprintf("Tap your %ssecurity key to complete login", promptDevicePrefix)
			switch {
			case quiet:
				// Do not prompt.
				prompt.FirstTouchMessage = ""
				prompt.SecondTouchMessage = ""
			case hasTOTP: // Webauthn + OTP
				prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key or enter a code from a %sOTP device", promptDevicePrefix, promptDevicePrefix)

				// Customize Windows prompt directly.
				// Note that the platform popup is a modal and will only go away if
				// canceled.
				webauthnwin.PromptPlatformMessage = "Follow the OS dialogs for platform authentication, or enter an OTP code here:"
				defer webauthnwin.ResetPromptPlatformMessage()

			default: // Webauthn only
				prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key", promptDevicePrefix)
			}
			mfaPrompt := &mfaPrompt{LoginPrompt: prompt, otpCancelAndWait: func() {
				otpCancel()
				otpWait.Wait()
			}}

			resp, _, err := p.WebauthnLogin(ctx, origin, wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge), mfaPrompt, &wancli.LoginOpts{
				AuthenticatorAttachment: p.AuthenticatorAttachment,
			})
			respC <- response{kind: "WEBAUTHN", resp: resp, err: err}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		select {
		case resp := <-respC:
			switch err := resp.err; {
			case errors.Is(err, wancli.ErrUsingNonRegisteredDevice):
				// Surface error immediately.
			case err != nil:
				log.WithError(err).Debugf("%s authentication failed", resp.kind)
				continue
			}

			// Cleanup in-flight goroutines.
			cancelAndWait()
			return resp.resp, trace.Wrap(resp.err)
		case <-ctx.Done():
			cancelAndWait()
			return nil, trace.Wrap(ctx.Err())
		}
	}
	cancelAndWait()
	return nil, trace.BadParameter(
		"failed to authenticate using all MFA devices, rerun the command with '-d' to see error details for each device")
}

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
