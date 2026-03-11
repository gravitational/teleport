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
	"github.com/gravitational/teleport/api/types"
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

	totpDeviceType     = "TOTP"
	webauthnDeviceType = "WEBAUTHN"
	touchIDDeviceType  = "TOUCHID"
)

var (
	totpDeviceTypes = []string{totpDeviceType}
	webDeviceTypes  = initWebDevs()

	// DefaultDeviceTypes lists the supported device types for `tsh mfa add`.
	DefaultDeviceTypes = append(totpDeviceTypes, webDeviceTypes...)
)

func initWebDevs() []string {
	if touchid.IsAvailable() {
		return []string{webauthnDeviceType, touchIDDeviceType}
	}
	return []string{webauthnDeviceType}
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
	// StdinFunc allows tests to override prompt.Stdin().
	// If nil prompt.Stdin() is used.
	StdinFunc  func() prompt.StdinReader
	StdoutFunc func() io.Writer
	RootClient ClusterClient
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
	if c.cfg.StdoutFunc == nil {
		return os.Stdout
	}
	return c.cfg.StdoutFunc()
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
	promptSSO := chal.SSOChallenge != nil

	// No prompt to run, no-op.
	if !promptOTP && !promptWebauthn && !promptSSO {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	isPerSessionMFA := c.cfg.Extensions.GetScope() == mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION
	var availableMethods []string
	if promptWebauthn {
		availableMethods = append(availableMethods, cliMFATypeWebauthn)
	}
	if promptSSO {
		availableMethods = append(availableMethods, cliMFATypeSSO)
	}
	if promptOTP && !isPerSessionMFA {
		availableMethods = append(availableMethods, cliMFATypeOTP)
	}

	// Check off unsupported methods.
	if promptWebauthn && !c.cfg.WebauthnSupported {
		promptWebauthn = false
		slog.DebugContext(ctx, "hardware device MFA not supported by your platform")
	}

	if promptSSO && c.cfg.SSOMFACeremony == nil {
		promptSSO = false
		slog.DebugContext(ctx, "SSO MFA not supported by this client, this is likely a bug")
	}

	// Short circuit if OTP was preferred by --mfa-mode during per-session MFA
	if c.cfg.PreferOTP && promptOTP && isPerSessionMFA {
		return nil, trace.AccessDenied("only WebAuthn and SSO MFA methods are supported with per-session MFA, can not specify --mfa-mode=otp")
	}

	// Prefer whatever method is requested by the client.
	var chosenMethods []string
	var userSpecifiedMethod bool
	switch {
	case c.cfg.PreferSSO && promptSSO:
		chosenMethods = []string{cliMFATypeSSO}
		promptWebauthn, promptOTP = false, false
		userSpecifiedMethod = true
	case c.cfg.PreferOTP && promptOTP:
		chosenMethods = []string{cliMFATypeOTP}
		promptWebauthn, promptSSO = false, false
		userSpecifiedMethod = true
	case c.cfg.AuthenticatorAttachment != wancli.AttachmentAuto:
		chosenMethods = []string{cliMFATypeWebauthn}
		promptSSO, promptOTP = false, false
		userSpecifiedMethod = true
	}

	// Use stronger auth methods if hijack is not allowed.
	if !c.cfg.AllowStdinHijack && promptWebauthn {
		promptOTP = false
	}

	// If we have multiple viable options, prefer Webauthn > SSO > OTP.
	switch {
	case promptWebauthn:
		chosenMethods = []string{cliMFATypeWebauthn}
		promptSSO = false
		// Allow dual prompt with OTP.
		if promptOTP {
			chosenMethods = append(chosenMethods, cliMFATypeOTP)
		}
	case promptSSO:
		chosenMethods = []string{cliMFATypeSSO}
		promptOTP = false
	case promptOTP:
		chosenMethods = []string{cliMFATypeOTP}
	}

	// If there are multiple options and we chose one without it being specifically
	// requested by the user, notify the user about it and how to request a specific method.
	if len(availableMethods) > len(chosenMethods) && len(chosenMethods) > 0 && !userSpecifiedMethod {
		availableMethodsString := strings.ToLower(strings.Join(availableMethods, ","))
		const msg = "" +
			"Available MFA methods [%v]. Continuing with %v.\n" +
			"If you wish to perform MFA with another method, specify with flag --mfa-mode=<%v> or environment variable TELEPORT_MFA_MODE=<%v>.\n\n"
		fmt.Fprintf(c.writer(), msg, strings.Join(availableMethods, ", "), strings.Join(chosenMethods, " and "), availableMethodsString, availableMethodsString)
	}

	// We should never prompt for OTP when per-session MFA is enabled. As long as other MFA methods are available,
	// we can completely ignore OTP. The promptOTP case below will return an error in the case that no other methods
	// are available.
	if isPerSessionMFA && (promptWebauthn || promptSSO) {
		promptOTP = false
	}

	switch {
	case promptOTP && promptWebauthn:
		resp, err := c.promptWebauthnAndOTP(ctx, chal)
		return resp, trace.Wrap(err)
	case promptWebauthn:
		resp, err := c.promptWebauthn(ctx, chal, c.getWebauthnPrompt(ctx))
		return resp, trace.Wrap(err)
	case promptSSO:
		resp, err := c.promptSSO(ctx, chal)
		return resp, trace.Wrap(err)
	case promptOTP:
		if isPerSessionMFA {
			return nil, trace.AccessDenied("only WebAuthn and SSO MFA methods are supported with per-session MFA")
		}

		resp, err := c.promptOTP(ctx, c.cfg.Quiet)
		return resp, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("client does not support any available MFA methods [%v], see debug logs for details", strings.Join(availableMethods, ", "))
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

func (c *CLIPrompt) AddMFA(ctx context.Context, spec mfa.MFASpec) (bool, error) {
	yes, err := prompt.Confirmation(ctx, c.stdout(), prompt.Stdin(),
		"\nYou have no MFA devices registered. Do you want to register a new one?",
	)
	if err != nil {
		return false, err
	}
	if !yes {
		return false, nil
	}

	// Attempt to diagnose clamshell failures.
	if !slices.Contains(DefaultDeviceTypes, touchIDDeviceType) {
		diag, err := touchid.Diag()
		if err == nil && diag.IsClamshellFailure() {
			slog.WarnContext(ctx, "Touch ID support disabled, is your MacBook lid closed?")
		}
	}

	if spec.DevType == "" {
		var err error
		spec.DevType, err = prompt.PickOne(
			ctx, os.Stdout, prompt.Stdin(),
			"Choose device type", deviceTypesFromSecondFactor(spec.AuthSecondFactor))
		if err != nil {
			return false, trace.Wrap(err)
		}
	}

	if spec.DevName == "" {
		var err error
		spec.DevName, err = prompt.Input(ctx, os.Stdout, prompt.Stdin(), "Enter device name")
		if err != nil {
			return false, trace.Wrap(err)
		}
	}
	spec.DevName = strings.TrimSpace(spec.DevName)
	if spec.DevName == "" {
		return false, trace.BadParameter("device name cannot be empty")
	}

	switch spec.DevType {
	case webauthnDeviceType:
		// Ask the user?
		if !spec.AllowPasswordlessSet && wancli.IsFIDO2Available() {
			answer, err := prompt.PickOne(ctx, os.Stdout, prompt.Stdin(), "Allow passwordless logins", []string{"YES", "NO"})
			if err != nil {
				return false, trace.Wrap(err)
			}
			spec.AllowPasswordless = answer == "YES"
		}
	case touchIDDeviceType:
		// Touch ID is always a resident key/passwordless
		spec.AllowPasswordless = true
	}
	slog.DebugContext(ctx, "tsh using passwordless registration?", "allow_passwordless", spec.AllowPasswordless)

	dev, err := c.addDeviceRPC(ctx, spec)
	if err != nil {
		return false, trace.Wrap(err)
	}

	fmt.Printf("MFA device %q added.\n", dev.Metadata.Name)
	return dev != nil, nil
}

func deviceTypesFromSecondFactor(sf constants.SecondFactorType) []string {
	switch sf {
	case constants.SecondFactorOTP:
		return totpDeviceTypes
	case constants.SecondFactorWebauthn:
		return webDeviceTypes
	default:
		return DefaultDeviceTypes
	}
}

func (c *CLIPrompt) addDeviceRPC(ctx context.Context, spec mfa.MFASpec) (*types.MFADevice, error) {
	devTypePB := map[string]proto.DeviceType{
		totpDeviceType:     proto.DeviceType_DEVICE_TYPE_TOTP,
		webauthnDeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		touchIDDeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}[spec.DevType]
	// Sanity check.
	if devTypePB == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected device type: %q", spec.DevType)
	}

	var dev *types.MFADevice
	// if err := client.RetryWithRelogin(ctx, tc, func() error {
	// clusterClient, err := tc.ConnectToCluster(ctx)
	// if err != nil {
	// 	return trace.Wrap(err)
	// }
	// defer clusterClient.Close()
	// rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
	// if err != nil {
	// 	return trace.Wrap(err)
	// }
	// defer rootAuthClient.Close()

	// TODO(awly): mfa: move this logic somewhere under /lib/auth/, closer
	// to the server logic. The CLI layer should ideally be thin.

	usage := proto.DeviceUsage_DEVICE_USAGE_MFA
	if spec.AllowPasswordless {
		usage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
	}

	// Tweak Windows platform messages so it's clear we whether we are prompting
	// for the *registered* or *new* device.
	// We do it here, preemptively, because it's the simpler solution (instead
	// of finding out whether it is a Windows prompt or not).
	//
	// TODO(Joerger): this should live in lib/client/mfa/cli.go using the prompt device prefix.
	const registeredMsg = "Using platform authentication for *registered* device, follow the OS dialogs"
	const newMsg = "Using platform authentication for *new* device, follow the OS dialogs"
	webauthnwin.SetPromptPlatformMessage(registeredMsg)
	defer webauthnwin.ResetPromptPlatformMessage()

	mfaResp, err := c.cfg.Ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Issue the registration challenge.
	registerChallenge, err := c.cfg.RootClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: mfaResp,
		DeviceType:          devTypePB,
		DeviceUsage:         usage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Prompt for registration.
	webauthnwin.SetPromptPlatformMessage(newMsg)
	registerResp, registerCallback, err := c.promptRegisterChallenge(ctx, c.cfg.ProxyAddress, spec.DevType, registerChallenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Complete registration and confirm new key.
	addResp, err := c.cfg.RootClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
		NewDeviceName:  spec.DevName,
		NewMFAResponse: registerResp,
		DeviceUsage:    usage,
	})
	if err != nil {
		registerCallback.Rollback() // Attempt to delete new key.
		return nil, trace.Wrap(err)
	}
	if err := registerCallback.Confirm(); err != nil {
		return nil, trace.Wrap(err)
	}

	dev = addResp.Device
	// return nil
	// }); err != nil {
	// 	return nil, trace.Wrap(err)
	// }
	return dev, nil
}

type registerCallback interface {
	Rollback() error
	Confirm() error
}

type noopRegisterCallback struct{}

func (n noopRegisterCallback) Rollback() error {
	return nil
}

func (n noopRegisterCallback) Confirm() error {
	return nil
}

func (c *CLIPrompt) promptRegisterChallenge(ctx context.Context, proxyAddr, devType string, rc *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, registerCallback, error) {
	switch rc.Request.(type) {
	case *proto.MFARegisterChallenge_TOTP:
		resp, err := promptTOTPRegisterChallenge(ctx, rc.GetTOTP())
		return resp, noopRegisterCallback{}, err

	case *proto.MFARegisterChallenge_Webauthn:
		origin := proxyAddr
		if !strings.HasPrefix(proxyAddr, "https://") {
			origin = "https://" + origin
		}
		cc := wantypes.CredentialCreationFromProto(rc.GetWebauthn())

		if devType == touchIDDeviceType {
			return promptTouchIDRegisterChallenge(origin, cc)
		}

		resp, err := c.promptWebauthnRegisterChallenge(ctx, origin, cc)
		return resp, noopRegisterCallback{}, err

	default:
		return nil, nil, trace.BadParameter("server bug: unexpected registration challenge type: %T", rc.Request)
	}
}

func promptTOTPRegisterChallenge(ctx context.Context, c *proto.TOTPRegisterChallenge) (*proto.MFARegisterResponse, error) {
	secretBin, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(c.Secret)
	if err != nil {
		return nil, trace.BadParameter("server sent an invalid TOTP secret key %q: %v", c.Secret, err)
	}
	var algorithm otp.Algorithm
	switch strings.ToUpper(c.Algorithm) {
	case "SHA1":
		algorithm = otp.AlgorithmSHA1
	case "SHA256":
		algorithm = otp.AlgorithmSHA256
	case "SHA512":
		algorithm = otp.AlgorithmSHA512
	case "MD5":
		algorithm = otp.AlgorithmMD5
	default:
		return nil, trace.BadParameter("server sent an unknown TOTP algorithm %q", c.Algorithm)
	}
	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      c.Issuer,
		AccountName: c.Account,
		Period:      uint(c.PeriodSeconds),
		Secret:      secretBin,
		Digits:      otp.Digits(c.Digits),
		Algorithm:   algorithm,
	})
	if err != nil {
		return nil, trace.BadParameter("server sent invalid TOTP parameters: %v", err)
	}

	// Try to show a QR code in the system image viewer.
	// This is not supported on all platforms.
	var showingQRCode bool
	closeQR, err := showOTPQRCode(otpKey)
	if err != nil {
		slog.DebugContext(ctx, "Failed to show QR code", "error", err)
	} else {
		showingQRCode = true
		defer closeQR()
	}

	fmt.Println()
	if showingQRCode {
		fmt.Println("Open your TOTP app and scan the QR code. Alternatively, you can manually enter these fields:")
	} else {
		fmt.Println("Open your TOTP app and create a new manual entry with these fields:")
	}
	fmt.Printf(`  URL: %s
  Account name: %s
  Secret key: %s
  Issuer: %s
  Algorithm: %s
  Number of digits: %d
  Period: %ds
`, otpKey.URL(), c.Account, c.Secret, c.Issuer, c.Algorithm, c.Digits, c.PeriodSeconds)
	fmt.Println()

	var totpCode string
	// Help the user with typos, don't submit the code until it has the right
	// length.
	for {
		totpCode, err = prompt.Password(
			ctx, os.Stdout, prompt.Stdin(), "Once created, enter an OTP code generated by the app")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(totpCode) == int(c.Digits) {
			break
		}
		fmt.Printf("TOTP code must be exactly %d digits long, try again\n", c.Digits)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{
				Code: totpCode,
				ID:   c.ID,
			},
		},
	}, nil
}

func (c *CLIPrompt) promptWebauthnRegisterChallenge(ctx context.Context, origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	slog.DebugContext(ctx, "prompting MFA devices with origin",
		teleport.ComponentKey, "WebAuthn",
		"origin", origin,
	)

	prompt := wancli.NewDefaultPrompt(ctx, os.Stdout)
	prompt.PINMessage = "Enter your *new* security key PIN"
	prompt.FirstTouchMessage = "Tap your *new* security key"
	prompt.SecondTouchMessage = "Tap your *new* security key again to complete registration"

	resp, err := c.cfg.WebauthnRegisterFunc(ctx, origin, cc, prompt)
	return resp, trace.Wrap(err)
}

func promptTouchIDRegisterChallenge(origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, registerCallback, error) {
	slog.DebugContext(context.TODO(), "prompting registration with origin",
		teleport.ComponentKey, "TouchID",
		"origin", origin,
	)

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

func showOTPQRCode(k *otp.Key) (cleanup func(), retErr error) {
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
	ctx := context.TODO()
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
