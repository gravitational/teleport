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
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

// CLIPromptConfig contains CLI prompt config options.
type CLIPromptConfig struct {
	PromptConfig
	// Writer is where the prompt outputs the prompt. Defaults to os.Stderr.
	Writer io.Writer
	// AllowStdinHijack allows stdin hijack during MFA prompts.
	// Stdin hijack provides a better login UX, but it can be difficult to reason
	// about and is often a source of bugs.
	// Do not set this options unless you deeply understand what you are doing.
	// If false then only the strongest auth method is prompted.
	AllowStdinHijack bool
	// PreferOTP favors OTP challenges, if applicable.
	// Takes precedence over AuthenticatorAttachment settings.
	PreferOTP bool
	// StdinFunc allows tests to override prompt.Stdin().
	// If nil prompt.Stdin() is used.
	StdinFunc func() prompt.StdinReader
}

// CLIPrompt is the default CLI mfa prompt implementation.
type CLIPrompt struct {
	cfg CLIPromptConfig
}

// NewCLIPrompt returns a new CLI mfa prompt with the given config.
func NewCLIPrompt(cfg *CLIPromptConfig) *CLIPrompt {
	// If no config is provided, use defaults (zero value).
	if cfg == nil {
		cfg = new(CLIPromptConfig)
	}
	return &CLIPrompt{
		cfg: *cfg,
	}
}

func (c *CLIPrompt) stdin() prompt.StdinReader {
	if c.cfg.StdinFunc == nil {
		return prompt.Stdin()
	}
	return c.cfg.StdinFunc()
}

func (c *CLIPrompt) writer() io.Writer {
	if c.cfg.Writer == nil {
		return os.Stderr
	}
	return c.cfg.Writer
}

// Run prompts the user to complete an MFA authentication challenge.
func (c *CLIPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if c.cfg.PromptReason != "" {
		fmt.Fprintln(c.writer(), c.cfg.PromptReason)
	}

	promptOTP := chal.TOTP != nil
	promptWebauthn := chal.WebauthnChallenge != nil

	// No prompt to run, no-op.
	if !promptOTP && !promptWebauthn {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	// Check off unsupported methods.
	if promptWebauthn && !c.cfg.WebauthnSupported {
		promptWebauthn = false
		slog.DebugContext(ctx, "hardware device MFA not supported by your platform")
		if !promptOTP {
			return nil, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
		}
	}

	// Prefer whatever method is requested by the client.
	if c.cfg.PreferOTP && promptOTP {
		promptWebauthn = false
	}

	// Use stronger auth methods if hijack is not allowed.
	if !c.cfg.AllowStdinHijack && promptWebauthn {
		promptOTP = false
	}

	// If a specific webauthn attachment was requested, skip OTP.
	// Otherwise, allow dual prompt with OTP.
	if promptWebauthn && c.cfg.AuthenticatorAttachment != wancli.AttachmentAuto {
		promptOTP = false
	}

	switch {
	case promptOTP && promptWebauthn:
		resp, err := c.promptWebauthnAndOTP(ctx, chal)
		return resp, trace.Wrap(err)
	case promptWebauthn:
		resp, err := c.promptWebauthn(ctx, chal, c.getWebauthnPrompt(ctx))
		return resp, trace.Wrap(err)
	case promptOTP:
		resp, err := c.promptOTP(ctx, c.cfg.Quiet)
		return resp, trace.Wrap(err)
	default:
		// We shouldn't reach this case as we would have hit the no-op case above.
		return nil, trace.BadParameter("no MFA methods to prompt")
	}
}

func (c *CLIPrompt) promptOTP(ctx context.Context, quiet bool) (*proto.MFAAuthenticateResponse, error) {
	var msg string
	if !quiet {
		msg = fmt.Sprintf("Enter an OTP code from a %sdevice", c.promptDevicePrefix())
	}

	otp, err := prompt.Password(ctx, c.writer(), c.stdin(), msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: otp},
		},
	}, nil
}

func (c *CLIPrompt) getWebauthnPrompt(ctx context.Context) *wancli.DefaultPrompt {
	writer := c.writer()
	if c.cfg.Quiet {
		writer = io.Discard
	}

	prompt := wancli.NewDefaultPrompt(ctx, writer)
	prompt.StdinFunc = c.cfg.StdinFunc
	prompt.SecondTouchMessage = fmt.Sprintf("Tap your %ssecurity key to complete login", c.promptDevicePrefix())
	prompt.FirstTouchMessage = fmt.Sprintf("Tap any %ssecurity key", c.promptDevicePrefix())
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

func (c *CLIPrompt) promptWebauthnAndOTP(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	spawnGoroutines := func(ctx context.Context, wg *sync.WaitGroup, respC chan<- MFAGoroutineResponse) {
		var message string
		if runtime.GOOS == constants.WindowsOS {
			message = "Follow the OS dialogs for platform authentication, or enter an OTP code here:"
			webauthnwin.SetPromptPlatformMessage("")
		} else {
			message = fmt.Sprintf("Tap any %ssecurity key or enter a code from a %sOTP device", c.promptDevicePrefix(), c.promptDevicePrefix())
		}
		fmt.Fprintln(c.writer(), message)

		// Fire OTP goroutine.
		var otpCancelAndWait func()
		otpCtx, otpCancel := context.WithCancel(ctx)
		otpDone := make(chan struct{})
		otpCancelAndWait = func() {
			otpCancel()
			<-otpDone
		}

		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				otpCancel()
				close(otpDone)
			}()

			resp, err := c.promptOTP(otpCtx, true /*quiet*/)
			respC <- MFAGoroutineResponse{Resp: resp, Err: trace.Wrap(err, "TOTP authentication failed")}
		}()

		// Fire Webauthn goroutine.
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				// Important for dual-prompt.
				webauthnwin.ResetPromptPlatformMessage()
			}()

			// Skip FirstTouchMessage when both OTP and WebAuthn are possible,
			// as the prompt happens externally.
			defaultPrompt := c.getWebauthnPrompt(ctx)
			defaultPrompt.FirstTouchMessage = ""

			// Wrap the prompt with otp context handler.
			prompt := &webauthnPromptWithOTP{
				LoginPrompt:      defaultPrompt,
				otpCancelAndWait: otpCancelAndWait,
			}

			resp, err := c.promptWebauthn(ctx, chal, prompt)
			respC <- MFAGoroutineResponse{Resp: resp, Err: trace.Wrap(err, "Webauthn authentication failed")}
		}()
	}

	return HandleMFAPromptGoroutines(ctx, spawnGoroutines)
}

// webauthnPromptWithOTP implements wancli.LoginPrompt for MFA logins.
// In most cases authenticators shouldn't require PINs or additional touches for
// MFA, but the implementation exists in case we find some unusual
// authenticators out there.
type webauthnPromptWithOTP struct {
	wancli.LoginPrompt

	otpCancelAndWaitOnce sync.Once
	otpCancelAndWait     func()
}

func (w *webauthnPromptWithOTP) cancelOTP() {
	if w.otpCancelAndWait == nil {
		return
	}
	w.otpCancelAndWaitOnce.Do(w.otpCancelAndWait)
}

func (w *webauthnPromptWithOTP) PromptTouch() (wancli.TouchAcknowledger, error) {
	ack, err := w.LoginPrompt.PromptTouch()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func() error {
		err := ack()

		// Stop the OTP goroutine when the first touch is acknowledged.
		w.cancelOTP()

		return trace.Wrap(err)
	}, nil
}

func (w *webauthnPromptWithOTP) PromptPIN() (string, error) {
	// Stop the OTP goroutine before asking for PIN, in case it wasn't already
	// stopped through PromptTouch.
	w.cancelOTP()

	return w.LoginPrompt.PromptPIN()
}
