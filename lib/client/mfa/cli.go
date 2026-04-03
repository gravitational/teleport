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
	"cmp"
	"context"
	"encoding/base32"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// cliMFATypeOTP is the CLI display name for OTP.
	cliMFATypeOTP = "OTP"
	// cliMFATypeWebauthn is the CLI display name for Webauthn.
	cliMFATypeWebauthn = "WEBAUTHN"
	// cliMFATypeSSO is the CLI display name for SSO.
	cliMFATypeSSO = "SSO"
	// cliMFATypeBrowserMFA is the CLI display name for Browser MFA.
	cliMFATypeBrowserMFA = "BROWSER"
)

var (
	// totpDeviceTypes are device types available when the second factor option
	// is set to [constants.SecondFactorOff].
	totpDeviceTypes = []mfa.MFADeviceType{mfa.TOTPDeviceType}

	// webDeviceTypes are device types available when the second factor option is
	// set to [constants.SecondFactorWebauthn].
	webDeviceTypes = initWebDevs()

	// DefaultDeviceTypes lists the supported device types for `tsh mfa add`.
	DefaultDeviceTypes = append(totpDeviceTypes, webDeviceTypes...)
)

func initWebDevs() []mfa.MFADeviceType {
	if touchid.IsAvailable() {
		return []mfa.MFADeviceType{mfa.WebauthnDeviceType, mfa.TouchIDDeviceType}
	}
	return []mfa.MFADeviceType{mfa.WebauthnDeviceType}
}

type ClusterClient interface {
	CreateRegisterChallenge(ctx context.Context, in *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error)
	AddMFADeviceSync(ctx context.Context, in *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error)
}

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
	// RuntimeOS overrides runtime.GOOS. Intended for tests only.
	RuntimeOS           string
	Stdout              io.Writer
	RootClient          ClusterClient
	CeremonyConstructor func() *mfa.Ceremony
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

func (c *CLIPrompt) stdout() io.Writer {
	if c.cfg.Stdout == nil {
		return os.Stdout
	}
	return c.cfg.Stdout
}

func (c *CLIPrompt) writer() io.Writer {
	if c.cfg.Writer == nil {
		return os.Stderr
	}
	return c.cfg.Writer
}

func (c *CLIPrompt) getOS() string {
	return cmp.Or(c.cfg.RuntimeOS, runtime.GOOS)
}

// mfaPromptState represents which MFA methods are available to prompt.
type mfaPromptState struct {
	promptWebauthn bool
	promptSSO      bool
	promptOTP      bool
	promptBrowser  bool
}

// filterMFAMethods determines which MFA method(s) to prompt for based on available methods,
// user preferences, and configuration. It prints a message to the user if multiple methods
// are available and returns the filtered state and list of available methods.
func (c *CLIPrompt) filterMFAMethods(state mfaPromptState, isPerSessionMFA bool, availableMethods []string) (mfaPromptState, bool) {
	var chosenMethods []string
	var userSpecifiedMethod bool

	// Prefer whatever method is requested by the client.
	switch {
	case c.cfg.PreferBrowser && state.promptBrowser:
		chosenMethods = []string{cliMFATypeBrowserMFA}
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
			chosenMethods = []string{cliMFATypeBrowserMFA}
		case state.promptOTP:
			chosenMethods = []string{cliMFATypeOTP}
		}
	}

	// If there are multiple options and we chose fewer without explicit user preference,
	// notify the user about the available methods and how to select a specific one.
	if len(availableMethods) > len(chosenMethods) && len(chosenMethods) > 0 && !userSpecifiedMethod {
		availableMethodsString := strings.ToLower(strings.Join(availableMethods, ","))
		const msg = "" +
			"Available MFA methods [%v]. Continuing with %v.\r\n" +
			"If you wish to perform MFA with another method, specify with flag --mfa-mode=<%v> or environment variable TELEPORT_MFA_MODE=<%v>.\r\n\r\n"
		fmt.Fprintf(c.writer(), msg, strings.Join(availableMethods, ", "), strings.Join(chosenMethods, " and "), availableMethodsString, availableMethodsString)
	}

	return state, userSpecifiedMethod
}

// Run prompts the user to complete an MFA authentication challenge.
func (c *CLIPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if c.cfg.PromptReason != "" {
		fmt.Fprintf(c.writer(), "%s\r\n", c.cfg.PromptReason)
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

	// Build list of available methods from the challenge before filtering
	// out unsupported methods. This list is used in user-facing messages.
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
		availableMethods = append(availableMethods, cliMFATypeBrowserMFA)
	}

	// Check off unsupported methods.
	if state.promptWebauthn && !c.cfg.WebauthnSupported {
		state.promptWebauthn = false
		slog.DebugContext(ctx, "Disabling WebAuthn: hardware device MFA not supported by your platform")
	}

	if state.promptSSO && c.cfg.CallbackCeremony == nil {
		state.promptSSO = false
		slog.DebugContext(ctx, "Disabling SSO MFA: SSO MFA ceremony not available (this is likely a bug)")
	}

	if state.promptBrowser && (chal.WebauthnChallenge == nil || c.cfg.CallbackCeremony == nil) {
		state.promptBrowser = false
		slog.DebugContext(
			ctx,
			"Disabling Browser MFA: user needs at least one webauthn device and client needs to support SSO MFA Ceremony",
			"webauthn_available", chal.WebauthnChallenge != nil,
			"mfa_ceremony_available (if false, this is a bug)", c.cfg.CallbackCeremony != nil,
		)
	}

	// Short circuit if OTP was preferred by --mfa-mode during per-session MFA.
	if c.cfg.PreferOTP && state.promptOTP && isPerSessionMFA {
		return nil, trace.AccessDenied("only WebAuthn, SSO MFA, and Browser MFA methods are supported with per-session MFA, cannot specify --mfa-mode=otp")
	}

	// Determine which method(s) to use and print options if multiple are available.
	var userSpecifiedMethod bool
	state, userSpecifiedMethod = c.filterMFAMethods(state, isPerSessionMFA, availableMethods)

	// Perform MFA with automatic fallback to other methods on failure.
	// In order: WebAuthn > SSO > Browser MFA > OTP
	return c.promptWithFallback(ctx, chal, state, availableMethods, isPerSessionMFA, userSpecifiedMethod)
}

func (c *CLIPrompt) promptWithFallback(ctx context.Context, chal *proto.MFAAuthenticateChallenge, state mfaPromptState, availableMethods []string, isPerSessionMFA, userSpecifiedMethod bool) (*proto.MFAAuthenticateResponse, error) {
	var lastErr error

	// If the user is running Windows and hasn't marked Browser MFA as preferred,
	// skip Browser MFA. They will have access to the same MFA methods using the
	// WebAuthn.dll prompt.
	skipBrowserMFAFallback := false
	if state.promptBrowser && !c.cfg.PreferBrowser && c.getOS() == constants.WindowsOS {
		skipBrowserMFAFallback = true
		slog.DebugContext(ctx, "Skipping Browser MFA fallback on Windows (WebAuthn.dll provides same functionality)")
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
			currentMethod = cliMFATypeBrowserMFA
		case state.promptOTP:
			currentMethod = cliMFATypeOTP
		default:
			// No more methods to try.
			slog.DebugContext(ctx, "No more MFA methods to try",
				"last_error", lastErr,
				"available_methods", strings.Join(availableMethods, ", "),
			)
			if lastErr != nil {
				return nil, trace.Wrap(lastErr)
			}
			return nil, trace.BadParameter("client does not support any available MFA methods [%v], see debug logs for details", strings.Join(availableMethods, ", "))
		}

		// If we're retrying after a failure, inform the user.
		if lastErr != nil {
			fmt.Fprintf(c.writer(), "Attempting MFA authentication with %s\r\n", currentMethod)
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
				return nil, trace.AccessDenied("only WebAuthn, SSO MFA, and Browser MFA methods are supported with per-session MFA")
			}
			resp, err = c.promptOTP(ctx, c.cfg.Quiet)
		}

		// MFA successful
		if err == nil {
			slog.DebugContext(ctx, "MFA authentication successful", "method", currentMethod)
			return resp, nil
		}

		slog.ErrorContext(ctx, "MFA authentication failed",
			"method", currentMethod,
			"error", err,
			"user_specified", userSpecifiedMethod,
		)

		// Don't fall back if the user explicitly chose this method.
		if userSpecifiedMethod {
			return nil, trace.Wrap(err)
		}

		// Print error message about the failure.
		fmt.Fprintf(c.writer(), "MFA authentication with %s failed, check logs for details\r\n", currentMethod)

		// Don't fall back if the context is done (e.g. user canceled or request timed out).
		if ctx.Err() != nil {
			return nil, trace.Wrap(err)
		}

		// Disable the failed method and loop to try the next one.
		// Fallback only moves forward in priority order: WebAuthn > SSO > Browser MFA > OTP
		switch currentMethod {
		case cliMFATypeWebauthn:
			state.promptWebauthn = false
		case cliMFATypeSSO:
			state.promptSSO = false
		case cliMFATypeBrowserMFA:
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
	var message string
	if c.getOS() == constants.WindowsOS {
		message = "Follow the OS dialogs for platform authentication, or enter an OTP code here:"
		webauthnwin.SetPromptPlatformMessage("")
	} else {
		message = fmt.Sprintf("Tap any %ssecurity key or enter a code from a %sOTP device", c.promptDevicePrefix(), c.promptDevicePrefix())
	}
	fmt.Fprintf(c.writer(), "%s\r\n", message)

	// Prepare to fire OTP goroutine.
	otpCtx, otpCancel := context.WithCancel(ctx)
	defer otpCancel()

	otpDone := make(chan struct{})
	otpCancelAndWait := func() {
		otpCancel()
		<-otpDone
	}

	promptOTP := func(ctx context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		defer func() {
			otpCancel()
			close(otpDone)
		}()

		resp, err := c.promptOTP(otpCtx, true /*quiet*/)
		return resp, trace.Wrap(err, "TOTP authentication failed")
	}

	promptWebauthn := func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		defer func() {
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
		return resp, trace.Wrap(err, "Webauthn authentication failed")
	}

	return HandleConcurrentMFAPrompts(ctx, chal, promptOTP, promptWebauthn)
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
	// MFACeremony.Run can handle either SSO or Browser MFA. It defaults to SSO MFA,
	// but to be safe, copy and remove the Browser MFA challenge here.
	ssoChal := *chal
	ssoChal.BrowserMFAChallenge = nil
	resp, err := c.cfg.CallbackCeremony.Run(ctx, &ssoChal)
	return resp, trace.Wrap(err)
}

func (c *CLIPrompt) promptBrowser(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	// MFACeremony.Run can handle either SSO or Browser MFA. It defaults to SSO MFA,
	// so remove copy and remove the SSO challenge so Browser MFA is used.
	browserChal := *chal
	browserChal.SSOChallenge = nil
	resp, err := c.cfg.CallbackCeremony.Run(ctx, &browserChal)
	return resp, trace.Wrap(err)
}

// AskRegister prompts user for device details and registers a new MFA device.
func (c *CLIPrompt) AskRegister(ctx context.Context, config mfa.RegistrationPromptConfig) (*mfa.RegistrationResult, error) {
	if !config.Confirmed {
		yes, err := prompt.Confirmation(ctx, c.stdout(), c.stdin(),
			"\nYou have no MFA devices registered. Do you want to register a new one?",
		)
		if err != nil {
			return nil, err
		}
		if !yes {
			return nil, nil
		}
	}

	// Attempt to diagnose clamshell failures.
	if !slices.Contains(DefaultDeviceTypes, mfa.TouchIDDeviceType) {
		diag, err := touchid.Diag()
		if err == nil && diag.IsClamshellFailure() {
			slog.WarnContext(ctx, "Touch ID support disabled, is your MacBook lid closed?")
		}
	}

	if config.DeviceType == "" {
		var err error
		config.DeviceType, err = prompt.PickOne(
			ctx, c.stdout(), c.stdin(),
			"Choose device type", deviceTypesFromSecondFactor(config.AuthSecondFactor))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	devTypePB := map[mfa.MFADeviceType]proto.DeviceType{
		mfa.TOTPDeviceType:     proto.DeviceType_DEVICE_TYPE_TOTP,
		mfa.WebauthnDeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		mfa.TouchIDDeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}[config.DeviceType]
	// Sanity check.
	if devTypePB == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected device type: %q", config.DeviceType)
	}

	if config.DeviceName == "" {
		var err error
		config.DeviceName, err = prompt.Input(ctx, c.stdout(), c.stdin(), "Enter device name")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	config.DeviceName = strings.TrimSpace(config.DeviceName)
	if config.DeviceName == "" {
		return nil, trace.BadParameter("device name cannot be empty")
	}

	switch config.DeviceType {
	case mfa.WebauthnDeviceType:
		// Ask the user?
		if config.DeviceUsage == proto.DeviceUsage_DEVICE_USAGE_UNSPECIFIED && wancli.IsFIDO2Available() {
			answer, err := prompt.PickOne(ctx, c.stdout(), c.stdin(), "Allow passwordless logins", []string{"YES", "NO"})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if answer == "YES" {
				config.DeviceUsage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
			} else {
				config.DeviceUsage = proto.DeviceUsage_DEVICE_USAGE_MFA
			}
		}
	case mfa.TouchIDDeviceType:
		// Touch ID is always a resident key/passwordless
		config.DeviceUsage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
	}
	slog.DebugContext(ctx, "Determined usage for newly registered MFA device", "usage", config.DeviceUsage.String())

	// Tweak Windows platform messages so it's clear we whether we are prompting
	// for the *registered* or *new* device.
	// We do it here, preemptively, because it's the simpler solution (instead
	// of finding out whether it is a Windows prompt or not).
	const registeredMsg = "Using platform authentication for *registered* device, follow the OS dialogs"
	const newMsg = "Using platform authentication for *new* device, follow the OS dialogs"
	webauthnwin.SetPromptPlatformMessage(registeredMsg)
	defer webauthnwin.ResetPromptPlatformMessage()

	ceremony := c.cfg.MFACeremony
	mfaResp, err := ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	regChal, err := ceremony.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: mfaResp,
		DeviceType:          devTypePB,
		DeviceUsage:         config.DeviceUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Prompt for registration.
	webauthnwin.SetPromptPlatformMessage(newMsg)
	resp, callback, err := c.promptRegisterChallenge(ctx, c.cfg.ProxyAddress, config.DeviceType, regChal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &mfa.RegistrationResult{
		Config:    config,
		Response:  resp,
		Callbacks: callback,
	}, nil
}

// NotifyRegistrationSuccess notifies the user that the device registration was
// successful.
func (c *CLIPrompt) NotifyRegistrationSuccess(_ context.Context, config mfa.RegistrationPromptConfig) error {
	fmt.Fprintf(c.stdout(), "MFA device %q added.\n\n", config.DeviceName)
	return nil
}

func deviceTypesFromSecondFactor(sf constants.SecondFactorType) []mfa.MFADeviceType {
	switch sf {
	case constants.SecondFactorOTP:
		return totpDeviceTypes
	case constants.SecondFactorWebauthn:
		return webDeviceTypes
	default:
		return DefaultDeviceTypes
	}
}

type noopRegisterCallback struct{}

func (n noopRegisterCallback) Rollback() error {
	return nil
}

func (n noopRegisterCallback) Confirm() error {
	return nil
}

func (c *CLIPrompt) promptRegisterChallenge(
	ctx context.Context, proxyAddr string, devType mfa.MFADeviceType, rc *proto.MFARegisterChallenge,
) (*proto.MFARegisterResponse, mfa.RegistrationCallbacks, error) {
	switch rc.Request.(type) {
	case *proto.MFARegisterChallenge_TOTP:
		resp, err := c.promptTOTPRegisterChallenge(ctx, rc.GetTOTP())
		return resp, noopRegisterCallback{}, err

	case *proto.MFARegisterChallenge_Webauthn:
		origin := proxyAddr
		if !strings.HasPrefix(proxyAddr, "https://") {
			origin = "https://" + origin
		}
		cc := wantypes.CredentialCreationFromProto(rc.GetWebauthn())

		if devType == mfa.TouchIDDeviceType {
			return c.promptTouchIDRegisterChallenge(origin, cc)
		}

		resp, err := c.promptWebauthnRegisterChallenge(ctx, origin, cc)
		return resp, noopRegisterCallback{}, err

	default:
		return nil, nil, trace.BadParameter("server bug: unexpected registration challenge type: %T", rc.Request)
	}
}

func (c *CLIPrompt) promptTOTPRegisterChallenge(ctx context.Context, chal *proto.TOTPRegisterChallenge) (*proto.MFARegisterResponse, error) {
	secretBin, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(chal.Secret)
	if err != nil {
		return nil, trace.BadParameter("server sent an invalid TOTP secret key %q: %v", chal.Secret, err)
	}
	var algorithm otp.Algorithm
	switch strings.ToUpper(chal.Algorithm) {
	case "SHA1":
		algorithm = otp.AlgorithmSHA1
	case "SHA256":
		algorithm = otp.AlgorithmSHA256
	case "SHA512":
		algorithm = otp.AlgorithmSHA512
	case "MD5":
		algorithm = otp.AlgorithmMD5
	default:
		return nil, trace.BadParameter("server sent an unknown TOTP algorithm %q", chal.Algorithm)
	}
	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      chal.Issuer,
		AccountName: chal.Account,
		Period:      uint(chal.PeriodSeconds),
		Secret:      secretBin,
		Digits:      otp.Digits(chal.Digits),
		Algorithm:   algorithm,
	})
	if err != nil {
		return nil, trace.BadParameter("server sent invalid TOTP parameters: %v", err)
	}

	// Try to show a QR code in the system image viewer.
	// This is not supported on all platforms.
	var showingQRCode bool
	closeQR, err := showOTPQRCode(ctx, otpKey)
	if err != nil {
		slog.DebugContext(ctx, "Failed to show QR code", "error", err)
	} else {
		showingQRCode = true
		defer closeQR()
	}

	fmt.Fprintln(c.stdout())
	if showingQRCode {
		fmt.Fprintln(c.stdout(), "Open your TOTP app and scan the QR code. Alternatively, you can manually enter these fields:")
	} else {
		fmt.Fprintln(c.stdout(), "Open your TOTP app and create a new manual entry with these fields:")
	}
	fmt.Fprintf(c.stdout(), `  URL: %s
  Account name: %s
  Secret key: %s
  Issuer: %s
  Algorithm: %s
  Number of digits: %d
  Period: %ds
`, otpKey.URL(), chal.Account, chal.Secret, chal.Issuer, chal.Algorithm, chal.Digits, chal.PeriodSeconds)
	fmt.Fprintln(c.stdout())

	var totpCode string
	// Help the user with typos, don't submit the code until it has the right
	// length.
	for {
		totpCode, err = prompt.Password(
			ctx, c.stdout(), prompt.Stdin(), "Once created, enter an OTP code generated by the app")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(totpCode) == int(chal.Digits) {
			break
		}
		fmt.Fprintf(c.stdout(), "TOTP code must be exactly %d digits long, try again\n", chal.Digits)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{
				Code: totpCode,
				ID:   chal.ID,
			},
		},
	}, nil
}

func (c *CLIPrompt) promptWebauthnRegisterChallenge(ctx context.Context, origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	slog.DebugContext(ctx, "prompting MFA devices with origin",
		teleport.ComponentKey, "WebAuthn",
		"origin", origin,
	)

	prompt := wancli.NewDefaultPrompt(ctx, c.stdout())
	prompt.PINMessage = "Enter your *new* security key PIN"
	prompt.FirstTouchMessage = "Tap your *new* security key"
	prompt.SecondTouchMessage = "Tap your *new* security key again to complete registration"

	resp, err := c.cfg.WebauthnRegisterFunc(ctx, origin, cc, prompt)
	return resp, trace.Wrap(err)
}

func (c *CLIPrompt) promptTouchIDRegisterChallenge(origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, mfa.RegistrationCallbacks, error) {
	slog.DebugContext(context.TODO(), "prompting registration with origin",
		teleport.ComponentKey, "TouchID",
		"origin", origin,
	)

	registerFunc := c.cfg.TouchIDRegisterFunc
	if registerFunc != nil {
		resp, callback, err := registerFunc(origin, cc)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return resp, callback, nil
	}

	reg, err := touchid.Register(origin, cc)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(reg.CCR),
		},
	}, reg, nil
}

func showOTPQRCode(ctx context.Context, k *otp.Key) (cleanup func(), retErr error) {
	var imageViewer string
	// imageViewerArgs is used to send additional arguments to exec command.
	var imageViewerArgs []string
	switch runtime.GOOS {
	case "linux":
		imageViewer = "xdg-open"
	case "darwin":
		imageViewer = "open"
	case "windows":
		// On windows start and many other commands are not executable files,
		// rather internal commands of Command prompt. In order to use internal
		// command it need to executed as: `cmd.exe /c start filename`
		imageViewer = "cmd.exe"
		imageViewerArgs = []string{"/c", "start"}
	default:
		return func() {}, trace.NotImplemented("showing QR codes is not implemented on %s", runtime.GOOS)
	}

	otpImage, err := k.Image(456, 456)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imageFile, err := os.CreateTemp("", "teleport-otp-qr-code-*.png")
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer func() {
		if retErr != nil {
			imageFile.Close()
			os.Remove(imageFile.Name())
		}
	}()

	if err := png.Encode(imageFile, otpImage); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := imageFile.Close(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	slog.DebugContext(ctx, "Wrote OTP QR code to file", "file", imageFile.Name())

	cmd := exec.Command(imageViewer, append(imageViewerArgs, imageFile.Name())...)
	if err := cmd.Start(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	slog.DebugContext(ctx, "Opened QR code via image viewer", "image_viewer", imageViewer)
	return func() {
		if err := utils.RemoveSecure(imageFile.Name()); err != nil {
			slog.DebugContext(ctx, "Failed to clean up temporary QR code file",
				"file", imageFile.Name(),
				"error", err,
			)
		}
		if err := cmd.Process.Kill(); err != nil {
			slog.DebugContext(ctx, "Failed to stop the QR code image viewer", "error", err)
		}
	}, nil
}
