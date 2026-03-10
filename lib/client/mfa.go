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

package client

import (
	"context"
	"encoding/base32"
	"fmt"
	"os"
	"slices"
	"strings"

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
	"github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/client/sso"
)

// NewMFACeremony returns a new MFA ceremony configured for this client.
func (tc *TeleportClient) NewMFACeremony() *mfa.Ceremony {
	return &mfa.Ceremony{
		CreateAuthenticateChallenge: tc.createAuthenticateChallenge,
		PromptConstructor:           tc.NewMFAPrompt,
		SSOMFACeremonyConstructor:   tc.NewSSOMFACeremony,
	}
}

// createAuthenticateChallenge creates and returns MFA challenges for a users registered MFA devices.
func (tc *TeleportClient) createAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rootClient.CreateAuthenticateChallenge(ctx, req)
}

// WebauthnLoginFunc is a function that performs WebAuthn login.
// Mimics the signature of [webauthncli.Login].
type WebauthnLoginFunc = libmfa.WebauthnLoginFunc

// WebauthnRegisterFunc is a function that performs WebAuthn registration.
// Mimics the signature of [wancli.Register].
type WebauthnRegisterFunc = libmfa.WebauthnLoginFunc

// NewMFAPrompt creates a new MFA prompt from client settings.
func (tc *TeleportClient) NewMFAPrompt(opts ...mfa.PromptOpt) mfa.Prompt {
	cfg := tc.newPromptConfig(opts...)

	var prompt mfa.Prompt = libmfa.NewCLIPrompt(&libmfa.CLIPromptConfig{
		PromptConfig:     *cfg,
		Writer:           tc.Stderr,
		PreferOTP:        tc.PreferOTP,
		PreferSSO:        tc.PreferSSO,
		AllowStdinHijack: tc.AllowStdinHijack,
		StdinFunc:        tc.StdinFunc,
	})

	if tc.MFAPromptConstructor != nil {
		prompt = tc.MFAPromptConstructor(cfg)
	}

	return prompt
}

func (tc *TeleportClient) newPromptConfig(opts ...mfa.PromptOpt) *libmfa.PromptConfig {
	cfg := libmfa.NewPromptConfig(tc.WebProxyAddr, opts...)
	cfg.AuthenticatorAttachment = tc.AuthenticatorAttachment
	if tc.WebauthnLogin != nil {
		cfg.WebauthnLoginFunc = tc.WebauthnLogin
		cfg.WebauthnSupported = true
	}

	return cfg
}

// NewSSOMFACeremony creates a new SSO MFA ceremony.
func (tc *TeleportClient) NewSSOMFACeremony(ctx context.Context) (mfa.SSOMFACeremony, error) {
	rdConfig, err := tc.ssoRedirectorConfig(ctx, "" /*connectorDisplayName*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rd, err := sso.NewRedirector(rdConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a redirector for SSO MFA")
	}

	if tc.SSOMFACeremonyConstructor != nil {
		return tc.SSOMFACeremonyConstructor(rd), nil
	}

	return sso.NewCLIMFACeremony(rd), nil
}

type MFASpec = libmfa.MFASpec

func (tc *TeleportClient) AddMFA(ctx context.Context, spec MFASpec) error {
	if spec.DevType == "" {
		// If we are prompting the user for the device type, then take a glimpse at
		// server-side settings and adjust the options accordingly.
		pingResp, err := tc.Ping(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		spec.DevType, err = prompt.PickOne(
			ctx, os.Stdout, prompt.Stdin(),
			"Choose device type", deviceTypesFromSecondFactor(pingResp.Auth.SecondFactor))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if a.devName == "" {
		var err error
		a.devName, err = prompt.Input(ctx, os.Stdout, prompt.Stdin(), "Enter device name")
		if err != nil {
			return trace.Wrap(err)
		}
	}
	a.devName = strings.TrimSpace(a.devName)
	if a.devName == "" {
		return trace.BadParameter("device name cannot be empty")
	}

	switch a.devType {
	case webauthnDeviceType:
		// Ask the user?
		if !a.allowPasswordlessSet && wancli.IsFIDO2Available() {
			answer, err := prompt.PickOne(ctx, os.Stdout, prompt.Stdin(), "Allow passwordless logins", []string{"YES", "NO"})
			if err != nil {
				return trace.Wrap(err)
			}
			a.allowPasswordless = answer == "YES"
		}
	case touchIDDeviceType:
		// Touch ID is always a resident key/passwordless
		a.allowPasswordless = true
	}
	logger.DebugContext(ctx, "tsh using passwordless registration?", "allow_passwordless", a.allowPasswordless)

	dev, err := a.addDeviceRPC(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("MFA device %q added.\n", dev.Metadata.Name)
	return nil
}

func deviceTypesFromSecondFactor(sf constants.SecondFactorType) []string {
	switch sf {
	case constants.SecondFactorOTP:
		return totpDeviceTypes
	case constants.SecondFactorWebauthn:
		return webDeviceTypes
	default:
		return defaultDeviceTypes
	}
}

func (a *mfaAdder) addDeviceRPC(ctx context.Context, tc *client.TeleportClient) (*types.MFADevice, error) {
	devTypePB := map[string]proto.DeviceType{
		totpDeviceType:     proto.DeviceType_DEVICE_TYPE_TOTP,
		webauthnDeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		touchIDDeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}[a.devType]
	// Sanity check.
	if devTypePB == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected device type: %q", a.devType)
	}

	var dev *types.MFADevice
	if err := client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()
		rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootAuthClient.Close()

		// TODO(awly): mfa: move this logic somewhere under /lib/auth/, closer
		// to the server logic. The CLI layer should ideally be thin.

		usage := proto.DeviceUsage_DEVICE_USAGE_MFA
		if a.allowPasswordless {
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
		wanwin.SetPromptPlatformMessage(registeredMsg)
		defer wanwin.ResetPromptPlatformMessage()

		mfaResp, err := tc.NewMFACeremony().Run(ctx, &proto.CreateAuthenticateChallengeRequest{
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Issue the registration challenge.
		registerChallenge, err := rootAuthClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			ExistingMFAResponse: mfaResp,
			DeviceType:          devTypePB,
			DeviceUsage:         usage,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Prompt for registration.
		wanwin.SetPromptPlatformMessage(newMsg)
		registerResp, registerCallback, err := a.promptRegisterChallenge(ctx, tc.WebProxyAddr, a.devType, registerChallenge)
		if err != nil {
			return trace.Wrap(err)
		}

		// Complete registration and confirm new key.
		addResp, err := rootAuthClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
			NewDeviceName:  a.devName,
			NewMFAResponse: registerResp,
			DeviceUsage:    usage,
		})
		if err != nil {
			registerCallback.Rollback() // Attempt to delete new key.
			return trace.Wrap(err)
		}
		if err := registerCallback.Confirm(); err != nil {
			return trace.Wrap(err)
		}

		dev = addResp.Device
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}
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

func (a *mfaAdder) promptRegisterChallenge(ctx context.Context, proxyAddr, devType string, c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, registerCallback, error) {
	switch c.Request.(type) {
	case *proto.MFARegisterChallenge_TOTP:
		resp, err := promptTOTPRegisterChallenge(ctx, c.GetTOTP())
		return resp, noopRegisterCallback{}, err

	case *proto.MFARegisterChallenge_Webauthn:
		origin := proxyAddr
		if !strings.HasPrefix(proxyAddr, "https://") {
			origin = "https://" + origin
		}
		cc := wantypes.CredentialCreationFromProto(c.GetWebauthn())

		if devType == touchIDDeviceType {
			return promptTouchIDRegisterChallenge(origin, cc)
		}

		resp, err := a.promptWebauthnRegisterChallenge(ctx, origin, cc)
		return resp, noopRegisterCallback{}, err

	default:
		return nil, nil, trace.BadParameter("server bug: unexpected registration challenge type: %T", c.Request)
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
		logger.DebugContext(ctx, "Failed to show QR code", "error", err)
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

func (a *mfaAdder) promptWebauthnRegisterChallenge(ctx context.Context, origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	logger.DebugContext(ctx, "prompting MFA devices with origin",
		teleport.ComponentKey, "WebAuthn",
		"origin", origin,
	)

	prompt := wancli.NewDefaultPrompt(ctx, os.Stdout)
	prompt.PINMessage = "Enter your *new* security key PIN"
	prompt.FirstTouchMessage = "Tap your *new* security key"
	prompt.SecondTouchMessage = "Tap your *new* security key again to complete registration"

	resp, err := a.webauthnRegister(ctx, origin, cc, prompt)
	return resp, trace.Wrap(err)
}

func promptTouchIDRegisterChallenge(origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, registerCallback, error) {
	logger.DebugContext(context.TODO(), "prompting registration with origin",
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
