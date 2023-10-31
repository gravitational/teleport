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
	"io"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

// CLIPrompt is the default CLI mfa prompt.
type CLIPrompt struct {
	writer io.Writer
}

// NewCLIPrompt returns a new CLI mfa prompt with the given writer.
func NewCLIPrompt(writer io.Writer) *CLIPrompt {
	return &CLIPrompt{
		writer: writer,
	}
}

// PromptTOTP prompts for TOTP.
func (c *CLIPrompt) PromptTOTP(ctx context.Context, chal *proto.MFAAuthenticateChallenge, cfg PromptConfig) (*proto.MFAAuthenticateResponse, error) {
	var msg string
	if !cfg.Quiet {
		msg = fmt.Sprintf("Enter an OTP code from a %sdevice", cfg.promptDevicePrefix())
	}

	otp, err := prompt.Password(ctx, c.writer, prompt.Stdin(), msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: otp},
		},
	}, nil
}

// PromptWebauthn prompts for Webauthn.
func (c *CLIPrompt) PromptWebauthn(ctx context.Context, chal *proto.MFAAuthenticateChallenge, cfg PromptConfig) (*proto.MFAAuthenticateResponse, error) {
	resp, err := cfg.WebauthnLogin(ctx, chal, c.getDefaultWebauthnPrompt(ctx, cfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// PromptWebauthnAndTOTP prompts for Webauthn and TOTP.
func (c *CLIPrompt) PromptWebauthnAndTOTP(ctx context.Context, chal *proto.MFAAuthenticateChallenge, cfg PromptConfig) (*proto.MFAAuthenticateResponse, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup

	type response struct {
		kind string
		resp *proto.MFAAuthenticateResponse
		err  error
	}
	respC := make(chan response, 2)

	// Use variables below to cancel OTP reads and make sure the goroutine exited.
	otpCtx, otpCancel := context.WithCancel(ctx)
	defer otpCancel()
	otpDone := make(chan struct{})
	otpCancelAndWait := func() {
		otpCancel()
		<-otpDone
	}

	// Fire TOTP goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(otpDone)

		// Let Webauthn take the prompt below.
		otpCfg := cfg
		otpCfg.Quiet = true

		resp, err := c.PromptTOTP(otpCtx, chal, otpCfg)
		respC <- response{kind: "TOTP", resp: resp, err: err}
	}()

	// Fire Webauthn goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()

		prompt := c.getDefaultWebauthnPrompt(ctx, cfg)
		prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key or enter a code from a %sOTP device", cfg.promptDevicePrefix(), cfg.promptDevicePrefix())

		// Customize Windows prompt directly.
		// Note that the platform popup is a modal and will only go away if canceled.
		webauthnwin.PromptPlatformMessage = "Follow the OS dialogs for platform authentication, or enter an OTP code here:"
		defer webauthnwin.ResetPromptPlatformMessage()

		// wrap webauthn prompt with otp context handler.
		mfaPrompt := &webauthnPromptWithOTP{LoginPrompt: prompt, otpCancelAndWait: otpCancelAndWait}

		resp, err := cfg.WebauthnLogin(ctx, chal, mfaPrompt)
		respC <- response{kind: "WEBAUTHN", resp: resp, err: err}
	}()

	go func() {
		wg.Wait()
		close(respC)
	}()

	for resp := range respC {
		switch err := resp.err; {
		case errors.Is(err, wancli.ErrUsingNonRegisteredDevice):
			// Surface error immediately.
		case err != nil:
			log.WithError(err).Debugf("%s authentication failed", resp.kind)
			continue
		}

		// Cancel and wait for the other prompt.
		cancel()
		wg.Wait()

		return resp.resp, trace.Wrap(resp.err)
	}

	return nil, trace.BadParameter("failed to authenticate using available MFA devices, rerun the command with '-d' to see error details for each device")
}

func (c *CLIPrompt) getDefaultWebauthnPrompt(ctx context.Context, cfg PromptConfig) *wancli.DefaultPrompt {
	writer := c.writer
	if cfg.Quiet {
		writer = io.Discard
	}

	prompt := wancli.NewDefaultPrompt(ctx, writer)
	prompt.SecondTouchMessage = fmt.Sprintf("Tap your %ssecurity key to complete login", cfg.promptDevicePrefix())
	prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key", cfg.promptDevicePrefix())
	return prompt
}

// webauthnPromptWithOTP implements wancli.LoginPrompt for MFA logins.
// In most cases authenticators shouldn't require PINs or additional touches for
// MFA, but the implementation exists in case we find some unusual
// authenticators out there.
type webauthnPromptWithOTP struct {
	wancli.LoginPrompt
	otpCancelAndWait func()
}

func (w *webauthnPromptWithOTP) PromptPIN() (string, error) {
	// If we get to this stage, Webauthn PIN verification is underway.
	// Cancel otp goroutine so that it doesn't capture the PIN from stdin.
	w.otpCancelAndWait()
	return w.LoginPrompt.PromptPIN()
}
