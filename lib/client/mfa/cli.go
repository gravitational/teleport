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
	"strings"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

const (
	// cliMFATypeOTP is the CLI display name for OTP.
	cliMFATypeOTP = "OTP"
	// cliMFATypeWebauthn is the CLI display name for Webauthn.
	cliMFATypeWebauthn = "WEBAUTHN"
	// cliMFATypeSSO is the CLI display name for SSO.
	cliMFATypeSSO = "SSO"
	// cliMFATypeBrowser is the CLI display name for Browser MFA.
	cliMFATypeBrowser = "BROWSER"
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
	// PreferSSO favors SSO challenges, if applicable.
	// Takes precedence over AuthenticatorAttachment settings.
	PreferSSO bool
	// PreferBrowser favors browser-based WebAuthn challenges, if applicable.
	// Takes precedence over AuthenticatorAttachment settings.
	PreferBrowser bool
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

// mfaPromptState represents which MFA methods are available to prompt.
type mfaPromptState struct {
	promptWebauthn bool
	promptSSO      bool
	promptOTP      bool
	promptBrowser  bool
}

// selectMFAMethods determines which MFA method(s) to prompt for based on available methods,
// user preferences, and configuration. It prints a message to the user if multiple methods
// are available and returns the filtered state and list of available methods.
func (c *CLIPrompt) selectMFAMethods(state mfaPromptState, isPerSessionMFA bool) (mfaPromptState, []string, bool) {
	// Build list of available methods from input state.
	var availableMethods []string
	if state.promptWebauthn {
		availableMethods = append(availableMethods, cliMFATypeWebauthn)
	}
	if state.promptSSO {
		availableMethods = append(availableMethods, cliMFATypeSSO)
	}
	if state.promptOTP && !isPerSessionMFA {
		availableMethods = append(availableMethods, cliMFATypeOTP)
	}
	if state.promptBrowser {
		availableMethods = append(availableMethods, cliMFATypeBrowser)
	}

	var chosenMethods []string
	var userSpecifiedMethod bool

	// Prefer whatever method is requested by the client.
	switch {
	case c.cfg.PreferBrowser && state.promptBrowser:
		chosenMethods = []string{cliMFATypeBrowser}
		state.promptWebauthn, state.promptOTP, state.promptSSO = false, false, false
		userSpecifiedMethod = true
	case c.cfg.PreferSSO && state.promptSSO:
		chosenMethods = []string{cliMFATypeSSO}
		state.promptWebauthn, state.promptOTP, state.promptBrowser = false, false, false
		userSpecifiedMethod = true
	case c.cfg.PreferOTP && state.promptOTP:
		chosenMethods = []string{cliMFATypeOTP}
		state.promptWebauthn, state.promptSSO, state.promptBrowser = false, false, false
		userSpecifiedMethod = true
	case c.cfg.AuthenticatorAttachment != wancli.AttachmentAuto:
		chosenMethods = []string{cliMFATypeWebauthn}
		state.promptSSO, state.promptOTP, state.promptBrowser = false, false, false
		userSpecifiedMethod = true
	}

	// Use stronger auth methods if hijack is not allowed.
	if !c.cfg.AllowStdinHijack && state.promptWebauthn {
		state.promptOTP = false
	}

	// If no user preference was specified, set initial MFA preference
	if !userSpecifiedMethod {
		// We should never prompt for OTP when per-session MFA is enabled. As long as other MFA methods are available,
		// we can completely ignore OTP. The promptOTP case in [Run] will return an error in the case that no other methods
		// are available.
		if isPerSessionMFA && (state.promptWebauthn || state.promptSSO || state.promptBrowser) {
			state.promptOTP = false
		}

		// Determine initial method to show based on MFA hierarchy:
		// Webauthn > SSO > Browser > OTP.
		switch {
		case state.promptWebauthn:
			chosenMethods = []string{cliMFATypeWebauthn}
			// Allow dual prompt with OTP.
			if state.promptOTP {
				chosenMethods = append(chosenMethods, cliMFATypeOTP)
			}
		case state.promptSSO:
			chosenMethods = []string{cliMFATypeSSO}
		case state.promptBrowser:
			chosenMethods = []string{cliMFATypeBrowser}
		case state.promptOTP:
			chosenMethods = []string{cliMFATypeOTP}
		}
	}

	// If there are multiple options and we chose fewer without explicit user preference,
	// notify the user about the available methods and how to select a specific one.
	if len(availableMethods) > len(chosenMethods) && len(chosenMethods) > 0 && !userSpecifiedMethod {
		availableMethodsString := strings.ToLower(strings.Join(availableMethods, ","))
		const msg = "" +
			"Available MFA methods [%v]. Continuing with %v.\n" +
			"If you wish to perform MFA with specific method, specify with flag --mfa-mode=<%v> or environment variable TELEPORT_MFA_MODE=<%v>.\n\n"
		fmt.Fprintf(c.writer(), msg, strings.Join(availableMethods, ", "), strings.Join(chosenMethods, " and "), availableMethodsString, availableMethodsString)
	}

	return state, availableMethods, userSpecifiedMethod
}

// Run prompts the user to complete an MFA authentication challenge.
func (c *CLIPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if c.cfg.PromptReason != "" {
		fmt.Fprintln(c.writer(), c.cfg.PromptReason)
	}

	// Initialize prompt state from the challenge.
	state := mfaPromptState{
		promptOTP:      chal.TOTP != nil,
		promptWebauthn: chal.WebauthnChallenge != nil,
		promptSSO:      chal.SSOChallenge != nil,
		promptBrowser:  chal.BrowserMFAChallenge != nil,
	}

	// No prompt to run, no-op.
	if !state.promptOTP && !state.promptWebauthn && !state.promptSSO && !state.promptBrowser {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	isPerSessionMFA := c.cfg.Extensions.GetScope() == mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION

	// Check off unsupported methods.
	if state.promptWebauthn && !c.cfg.WebauthnSupported {
		state.promptWebauthn = false
		slog.DebugContext(ctx, "hardware device MFA not supported by your platform")
	}

	if state.promptSSO && c.cfg.SSOMFACeremony == nil {
		state.promptSSO = false
		slog.DebugContext(ctx, "SSO MFA not supported by this client, this is likely a bug")
	}

	if state.promptBrowser && (!c.cfg.WebauthnSupported || c.cfg.SSOMFACeremony == nil) {
		state.promptBrowser = false
		slog.DebugContext(
			ctx,
			"Browser MFA not supported, cluster needs to support Webauthn and client needs to support SSO MFA Ceremony",
			"Webauthn", c.cfg.WebauthnSupported,
			"SSO MFA Ceremony (likely a bug if not supported)", c.cfg.SSOMFACeremony != nil,
		)
	}

	// Short circuit if OTP was preferred by --mfa-mode during per-session MFA.
	if c.cfg.PreferOTP && state.promptOTP && isPerSessionMFA {
		return nil, trace.AccessDenied("only WebAuthn and SSO MFA methods are supported with per-session MFA, can not specify --mfa-mode=otp")
	}

	// Determine which method(s) to use and print options if multiple are available.
	var availableMethods []string
	var userSpecifiedMethod bool
	state, availableMethods, userSpecifiedMethod = c.selectMFAMethods(state, isPerSessionMFA)

	// Perform MFA with automatic fallback to other methods on failure.
	// In order: WebAuthn > SSO > Browser > OTP
	return c.promptWithFallback(ctx, chal, state, availableMethods, isPerSessionMFA, userSpecifiedMethod)
}

// promptWithFallback attempts MFA authentication and falls back to other available methods on failure.
func (c *CLIPrompt) promptWithFallback(ctx context.Context, chal *proto.MFAAuthenticateChallenge, state mfaPromptState, availableMethods []string, isPerSessionMFA, userSpecifiedMethod bool) (*proto.MFAAuthenticateResponse, error) {
	var lastErr error

	// If the user is running Windows and hasn't marked Browser MFA as preferred
	// skip Browser MFA. They will have access to the same MFA methods using the
	// WebAuthn.dll prompt.
	skipBrowserMFAFallback := false
	if state.promptBrowser && !c.cfg.PreferBrowser && runtime.GOOS == constants.WindowsOS {
		skipBrowserMFAFallback = true
	}

	// Retry loop for fallback behavior.
	for {
		// Determine current method to try based on priority order.
		var currentMethod string
		switch {
		case state.promptWebauthn:
			currentMethod = cliMFATypeWebauthn
		case state.promptSSO:
			currentMethod = cliMFATypeSSO
		case state.promptBrowser && !skipBrowserMFAFallback:
			currentMethod = cliMFATypeBrowser
		case state.promptOTP:
			currentMethod = cliMFATypeOTP
		default:
			// No more methods to try.
			if lastErr != nil {
				return nil, trace.Wrap(lastErr)
			}
			return nil, trace.BadParameter("client does not support any available MFA methods [%v], see debug logs for details", strings.Join(availableMethods, ", "))
		}

		// If we're retrying after a failure, inform the user.
		if lastErr != nil {
			fmt.Fprintf(c.writer(), "Attempting MFA authentication with %s...\n\n", currentMethod)
		}

		// Perform the chosen ceremony based on the filtered state.
		var resp *proto.MFAAuthenticateResponse
		var err error

		switch {
		case state.promptWebauthn:
			if state.promptOTP {
				resp, err = c.promptWebauthnAndOTP(ctx, chal)
			} else {
				resp, err = c.promptWebauthn(ctx, chal, c.getWebauthnPrompt(ctx))
			}
		case state.promptSSO:
			resp, err = c.promptSSO(ctx, chal)
		case state.promptBrowser && !skipBrowserMFAFallback:
			resp, err = c.promptBrowser(ctx, chal)
		case state.promptOTP:
			if isPerSessionMFA {
				return nil, trace.AccessDenied("only WebAuthn, SSO MFA, and Browser methods are supported with per-session MFA")
			}
			resp, err = c.promptOTP(ctx, c.cfg.Quiet)
		}

		// MFA successful
		if err == nil {
			return resp, nil
		}

		// If the user explicitly specified this MFA method, fail now without fallback.
		if userSpecifiedMethod {
			return nil, trace.Wrap(err)
		}

		// Print error message about the failure.
		fmt.Fprintf(c.writer(), "\nMFA authentication with %s failed: %v\n", currentMethod, err)

		// Disable the failed method and loop to try the next one.
		// Fallback only moves forward in priority order: WebAuthn > SSO > Browser > OTP
		switch currentMethod {
		case cliMFATypeWebauthn:
			state.promptWebauthn = false
		case cliMFATypeSSO:
			state.promptSSO = false
		case cliMFATypeBrowser:
			state.promptBrowser = false
		case cliMFATypeOTP:
			state.promptOTP = false
		}

		lastErr = err
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

func (c *CLIPrompt) promptSSO(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	resp, err := c.cfg.SSOMFACeremony.Run(ctx, chal)
	return resp, trace.Wrap(err)
}

func (c *CLIPrompt) promptBrowser(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	resp, err := c.cfg.SSOMFACeremony.Run(ctx, chal)
	return resp, trace.Wrap(err)
}
