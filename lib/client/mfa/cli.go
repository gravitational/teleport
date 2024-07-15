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

package mfa

import (
	"context"
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

// CLIPrompt is the default CLI mfa prompt implementation.
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
	if c.cfg.PromptReason != "" {
		fmt.Fprintln(c.writer, c.cfg.PromptReason)
	}

	runOpts, err := c.cfg.GetRunOptions(ctx, chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// No prompt to run, no-op.
	if !runOpts.PromptTOTP && !runOpts.PromptWebauthn {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	// Depending on the run opts, we may spawn a TOTP goroutine, webauth goroutine, or both.
	spawnGoroutines := func(ctx context.Context, wg *sync.WaitGroup, respC chan<- MFAGoroutineResponse) {
		// Use variables below to cancel OTP reads and make sure the goroutine exited.
		otpCtx, otpCancel := context.WithCancel(ctx)
		otpDone := make(chan struct{})
		otpCancelAndWait := func() {
			otpCancel()
			<-otpDone
		}

		// Fire TOTP goroutine.
		if runOpts.PromptTOTP {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer otpCancel()
				defer close(otpDone)

				// Let Webauthn take the prompt below if applicable.
				quiet := c.cfg.Quiet || runOpts.PromptWebauthn

				resp, err := c.promptTOTP(otpCtx, chal, quiet)
				respC <- MFAGoroutineResponse{Resp: resp, Err: trace.Wrap(err, "TOTP authentication failed")}
			}()
		}

		// Fire Webauthn goroutine.
		if runOpts.PromptWebauthn {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Get webauthn prompt and wrap with otp context handler.
				prompt := &webauthnPromptWithOTP{
					LoginPrompt:      c.getWebauthnPrompt(ctx, runOpts.PromptTOTP),
					otpCancelAndWait: otpCancelAndWait,
				}

				resp, err := c.promptWebauthn(ctx, chal, prompt)
				respC <- MFAGoroutineResponse{Resp: resp, Err: trace.Wrap(err, "Webauthn authentication failed")}
			}()
		}
	}

	return HandleMFAPromptGoroutines(ctx, spawnGoroutines)
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
	resp, _, err := c.cfg.WebauthnLoginFunc(ctx, c.cfg.GetWebauthnOrigin(), wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge), prompt, opts)
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
