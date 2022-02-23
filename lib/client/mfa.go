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
	"io"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

type (
	OTPPrompt func(ctx context.Context, out io.Writer, in *prompt.ContextReader, question string) (string, error)
	WebPrompt func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error)
)

// PlatformPrompt groups functions that prompt the user for inputs.
// It's purpose is to allow tests to replace actual user prompts with other
// functions.
type PlatformPrompt struct {
	// OTP is the OTP prompt function.
	OTP OTPPrompt
	// Webauthn is the WebAuth prompt function.
	Webauthn WebPrompt
}

func (pp *PlatformPrompt) Reset() *PlatformPrompt {
	pp.Swap(prompt.Input, wancli.Login)
	return pp
}

func (pp *PlatformPrompt) Swap(otp OTPPrompt, web WebPrompt) {
	pp.OTP = otp
	pp.Webauthn = web
}

var prompts = (&PlatformPrompt{}).Reset()

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
//
// If promptDevicePrefix is set, it will be printed in prompts before "security
// key" or "device". This is used to emphasize between different kinds of
// devices, like registered vs new.
func PromptMFAChallenge(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge, promptDevicePrefix string, quiet bool) (*proto.MFAAuthenticateResponse, error) {
	// Is there a challenge present?
	if c.TOTP == nil && c.WebauthnChallenge == nil {
		return &proto.MFAAuthenticateResponse{}, nil
	}

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

	// Fire TOTP goroutine.
	if hasTOTP {
		wg.Add(1)
		go func() {
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
			code, err := prompts.OTP(ctx, os.Stderr, prompt.Stdin(), msg)
			if err != nil {
				respC <- response{kind: kind, err: err}
				return
			}
			respC <- response{
				kind: kind,
				resp: &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{Code: code},
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
			resp, err := prompts.Webauthn(ctx, origin, wanlib.CredentialAssertionFromProto(c.WebauthnChallenge))
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

			if hasTOTP {
				fmt.Fprintln(os.Stderr) // Print a new line after the prompt
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

// MakeAuthenticateChallenge converts proto to JSON format.
func MakeAuthenticateChallenge(protoChal *proto.MFAAuthenticateChallenge) *auth.MFAAuthenticateChallenge {
	chal := &auth.MFAAuthenticateChallenge{
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
