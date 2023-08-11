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
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/utils/prompt"
)

// promptWebauthn provides indirection for tests.
var promptWebauthn = wancli.Login

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

// promptMFAStandalone is used to mock PromptMFAChallenge for tests.
var promptMFAStandalone = PromptMFAChallenge

// hasPlatformSupport is used to mock wancli.HasPlatformSupport for tests.
var hasPlatformSupport = wancli.HasPlatformSupport

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
// If proxyAddr is empty, the TeleportClient.WebProxyAddr is used.
// See client.PromptMFAChallenge.
func (tc *TeleportClient) PromptMFAChallenge(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge, applyOpts func(opts *PromptMFAChallengeOpts)) (*proto.MFAAuthenticateResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/PromptMFAChallenge",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", tc.SiteName),
			attribute.Bool("prefer_otp", tc.PreferOTP),
		),
	)
	defer span.End()

	addr := proxyAddr
	if addr == "" {
		addr = tc.WebProxyAddr
	}

	opts := &PromptMFAChallengeOpts{
		AuthenticatorAttachment: tc.AuthenticatorAttachment,
		PreferOTP:               tc.PreferOTP,
	}
	if applyOpts != nil {
		applyOpts(opts)
	}

	return promptMFAStandalone(ctx, c, addr, opts)
}

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
func PromptMFAChallenge(ctx context.Context, c *proto.MFAAuthenticateChallenge, proxyAddr string, opts *PromptMFAChallengeOpts) (*proto.MFAAuthenticateResponse, error) {
	ctx, span := tracing.NewTracer("mfa").Start(
		ctx,
		"PromptMFAChallenge",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// Is there a challenge present?
	if c.TOTP == nil && c.WebauthnChallenge == nil {
		return &proto.MFAAuthenticateResponse{}, nil
	}
	if opts == nil {
		opts = &PromptMFAChallengeOpts{}
	}
	writer := os.Stderr
	if opts.HintBeforePrompt != "" {
		fmt.Fprintln(writer, opts.HintBeforePrompt)
	}
	promptDevicePrefix := opts.PromptDevicePrefix
	quiet := opts.Quiet

	hasTOTP := c.TOTP != nil
	hasWebauthn := c.WebauthnChallenge != nil

	// Does the current platform support hardware MFA? Adjust accordingly.
	switch {
	case !hasTOTP && !hasPlatformSupport():
		return nil, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
	case !hasPlatformSupport():
		// Do not prompt for hardware devices, it won't work.
		hasWebauthn = false
	}

	// Tweak enabled/disabled methods according to opts.
	switch {
	case hasTOTP && opts.PreferOTP:
		hasWebauthn = false
	case hasWebauthn && opts.AuthenticatorAttachment != wancli.AttachmentAuto:
		// Prefer Webauthn if an specific attachment was requested.
		hasTOTP = false
	case hasWebauthn && !opts.AllowStdinHijack:
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
		origin := proxyAddr
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
			default: // Webauthn only
				prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key", promptDevicePrefix)
			}
			mfaPrompt := &mfaPrompt{LoginPrompt: prompt, otpCancelAndWait: func() {
				otpCancel()
				otpWait.Wait()
			}}

			resp, _, err := promptWebauthn(ctx, origin, wanlib.CredentialAssertionFromProto(c.WebauthnChallenge), mfaPrompt, &wancli.LoginOpts{
				AuthenticatorAttachment: opts.AuthenticatorAttachment,
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

// MFAAuthenticateChallenge is an MFA authentication challenge sent on user
// login / authentication ceremonies.
type MFAAuthenticateChallenge struct {
	// WebauthnChallenge contains a WebAuthn credential assertion used for
	// login/authentication ceremonies.
	WebauthnChallenge *wanlib.CredentialAssertion `json:"webauthn_challenge"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
}

// MakeAuthenticateChallenge converts proto to JSON format.
func MakeAuthenticateChallenge(protoChal *proto.MFAAuthenticateChallenge) *MFAAuthenticateChallenge {
	chal := &MFAAuthenticateChallenge{
		TOTPChallenge: protoChal.GetTOTP() != nil,
	}
	if protoChal.GetWebauthnChallenge() != nil {
		chal.WebauthnChallenge = wanlib.CredentialAssertionFromProto(protoChal.WebauthnChallenge)
	}
	return chal
}

type TOTPRegisterChallenge struct {
	QRCode []byte `json:"qrCode"`
}

// MFARegisterChallenge is an MFA register challenge sent on new MFA register.
type MFARegisterChallenge struct {
	// Webauthn contains webauthn challenge.
	Webauthn *wanlib.CredentialCreation `json:"webauthn"`
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
			Webauthn: wanlib.CredentialCreationFromProto(protoChal.GetWebauthn()),
		}
	}
	return nil
}
