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
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

// CLIPrompt is the default CLI mfa prompt.
type CLIPrompt struct {
	cfg    PromptConfig
	writer io.Writer
}

// NewCLIPrompt returns a new CLI mfa prompt with the config and writer.
func NewCLIPrompt(cfg *PromptConfig, writer io.Writer) *CLIPrompt {
	return &CLIPrompt{
		cfg:    *cfg,
		writer: writer,
	}
}

// Run prompts the user to complete an MFA authentication challenge.
func (c *CLIPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	runOpts, err := c.cfg.GetRunOptions(ctx, chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type response struct {
		kind string
		resp *proto.MFAAuthenticateResponse
		err  error
	}
	respC := make(chan response, 2)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		// wait for all goroutines to complete to ensure there are no leaks.
		wg.Wait()
	}()

	// Use variables below to cancel OTP reads and make sure the goroutine exited.
	otpCtx, otpCancel := context.WithCancel(ctx)
	defer otpCancel()
	otpDone := make(chan struct{})
	otpCancelAndWait := func() {
		otpCancel()
		<-otpDone
	}

	// Fire TOTP goroutine.
	if runOpts.promptTOTP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(otpDone)

			// Let Webauthn take the prompt below.
			resp, err := c.promptTOTP(otpCtx, chal, true /*quiet*/)
			respC <- response{kind: "TOTP", resp: resp, err: err}
		}()
	}

	// Fire Webauthn goroutine.
	if runOpts.promptWebauthn {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// get webauthn prompt and wrap with otp context handler.
			prompt := &webauthnPromptWithOTP{
				LoginPrompt:      c.getWebauthnPrompt(ctx, runOpts.promptTOTP),
				otpCancelAndWait: otpCancelAndWait,
			}

			resp, err := c.promptWebauthn(ctx, chal, prompt)
			respC <- response{kind: "WEBAUTHN", resp: resp, err: err}
		}()
	}

	// Wait for the 1-2 authn goroutines above to complete, then close respC.
	go func() {
		wg.Wait()
		close(respC)
	}()

	// Wait for a successful response, or terminating error, from the 1-2 authn goroutines.
	// The goroutine above will ensure the response channel is closed once all goroutines are done.
	for resp := range respC {
		switch err := resp.err; {
		case errors.Is(err, wancli.ErrUsingNonRegisteredDevice):
			// Surface error immediately.
			return nil, trace.Wrap(resp.err)
		case err != nil:
			c.cfg.Log.WithError(err).Debugf("%s authentication failed", resp.kind)
			// Continue to give the other authn goroutine a chance to succeed.
			// If both have failed, this will exit the loop.
			continue
		}

		// Return successful response.
		return resp.resp, nil
	}

	// If no successful response is returned, this means the authn goroutines were unsuccessful.
	// This usually occurs when the prompt times out or no devices are available to prompt.
	// Return a user readable error message.
	return nil, trace.BadParameter("failed to authenticate using available MFA devices, rerun the command with '-d' to see error details for each device")
}

func (c *CLIPrompt) promptTOTP(ctx context.Context, chal *proto.MFAAuthenticateChallenge, quiet bool) (*proto.MFAAuthenticateResponse, error) {
	var msg string
	if !quiet {
		msg = fmt.Sprintf("Enter an OTP code from a %sdevice", c.promptDevicePrefix())
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

func (c *CLIPrompt) getWebauthnPrompt(ctx context.Context, withTOTP bool) wancli.LoginPrompt {
	writer := c.writer
	if c.cfg.Quiet {
		writer = io.Discard
	}

	prompt := wancli.NewDefaultPrompt(ctx, writer)
	prompt.SecondTouchMessage = fmt.Sprintf("Tap your %ssecurity key to complete login", c.promptDevicePrefix())
	prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key", c.promptDevicePrefix())

	if withTOTP {
		prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key or enter a code from a %sOTP device", c.promptDevicePrefix(), c.promptDevicePrefix())

		// Customize Windows prompt directly.
		// Note that the platform popup is a modal and will only go away if canceled.
		webauthnwin.PromptPlatformMessage = "Follow the OS dialogs for platform authentication, or enter an OTP code here:"
		defer webauthnwin.ResetPromptPlatformMessage()
	}

	return prompt
}

func (c *CLIPrompt) promptWebauthn(ctx context.Context, chal *proto.MFAAuthenticateChallenge, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
	opts := &wancli.LoginOpts{AuthenticatorAttachment: c.cfg.AuthenticatorAttachment}
	resp, _, err := c.cfg.WebauthnLoginFunc(ctx, c.cfg.getWebauthnOrigin(), wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge), prompt, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (c *CLIPrompt) promptDevicePrefix() string {
	if c.cfg.DeviceType != "" {
		return fmt.Sprintf("*%s* ", c.cfg.DeviceType)
	}
	return ""
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
