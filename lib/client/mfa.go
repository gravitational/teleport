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
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
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
	// PromptDevicePrefix is an optional prefix printed before "security key" or
	// "device". It is used to emphasize between different kinds of devices, like
	// registered vs new.
	PromptDevicePrefix string
	// Quiet suppresses users prompts.
	Quiet bool
	// UseStrongestAuth prompts the user to solve only the strongest challenge
	// available.
	// If set it also avoids stdin hijacking, as only one prompt is necessary.
	UseStrongestAuth bool
}

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
// PromptMFAChallenge makes an attempt to read OTPs from prompt.Stdin and
// abandons the read if the user chooses WebAuthn instead. For this reason
// callers must use prompt.Stdin exclusively after calling this function.
// Set opts.UseStrongestAuth to avoid stdin hijacking.
func PromptMFAChallenge(ctx context.Context, c *proto.MFAAuthenticateChallenge, proxyAddr string, opts *PromptMFAChallengeOpts) (*proto.MFAAuthenticateResponse, error) {
	// Is there a challenge present?
	if c.TOTP == nil && c.WebauthnChallenge == nil {
		return &proto.MFAAuthenticateResponse{}, nil
	}
	if opts == nil {
		opts = &PromptMFAChallengeOpts{}
	}
	promptDevicePrefix := opts.PromptDevicePrefix
	quiet := opts.Quiet

	hasTOTP := c.TOTP != nil
	hasWebauthn := c.WebauthnChallenge != nil

	// Does the current platform support hardware MFA? Adjust accordingly.
	switch {
	case !hasTOTP && !wancli.HasPlatformSupport():
		return nil, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
	case !wancli.HasPlatformSupport():
		// Do not prompt for hardware devices, it won't work.
		hasWebauthn = false
	}

	// Prompt only for the strongest auth method available?
	if opts.UseStrongestAuth && hasWebauthn {
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
			var msg string
			if !quiet {
				if hasWebauthn {
					msg = fmt.Sprintf("Tap any %[1]ssecurity key or enter a code from a %[1]sOTP device", promptDevicePrefix, promptDevicePrefix)
				} else {
					msg = fmt.Sprintf("Enter an OTP code from a %sdevice", promptDevicePrefix)
				}
			}

			otp, err := prompt.Password(otpCtx, os.Stderr, prompt.Stdin(), msg)
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
	} else if !quiet {
		fmt.Fprintf(os.Stderr, "Tap any %ssecurity key\n", promptDevicePrefix)
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

			prompt := wancli.NewDefaultPrompt(ctx, os.Stderr)
			prompt.FirstTouchMessage = "" // First prompt printed above.
			prompt.SecondTouchMessage = fmt.Sprintf("Tap your %ssecurity key to complete login", promptDevicePrefix)
			mfaPrompt := &mfaPrompt{LoginPrompt: prompt, otpCancelAndWait: func() {
				otpCancel()
				otpWait.Wait()
			}}

			const user = ""
			resp, _, err := promptWebauthn(ctx, origin, wanlib.CredentialAssertionFromProto(c.WebauthnChallenge), mfaPrompt, &wancli.LoginOpts{
				User: user,
			})
			respC <- response{kind: "WEBAUTHN", resp: resp, err: err}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		select {
		case resp := <-respC:
			if err := resp.err; err != nil {
				log.WithError(err).Debugf("%s authentication failed", resp.kind)
				continue
			}

			// Cleanup in-flight goroutines.
			cancelAndWait()
			return resp.resp, nil
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
