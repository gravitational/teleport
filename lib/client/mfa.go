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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

// promptOTP allows tests to override the OTP prompt function.
var promptOTP = prompt.Input

// promptU2F allows tests to override the U2F prompt function.
var promptU2F = u2f.AuthenticateSignChallenge

// promptWebauthn allows tests to override the Webauthn prompt function.
var promptWebauthn = wancli.Login

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
//
// If promptDevicePrefix is set, it will be printed in prompts before "security
// key" or "device". This is used to emphasize between different kinds of
// devices, like registered vs new.
func PromptMFAChallenge(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge, promptDevicePrefix string, quiet bool) (*proto.MFAAuthenticateResponse, error) {
	// Is there a challenge present?
	if c.TOTP == nil && len(c.U2F) == 0 && c.WebauthnChallenge == nil {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	// We have three maximum challenges, from which we only pick two: TOTP and
	// either Webauthn (preferred) or U2F.
	hasTOTP := c.TOTP != nil
	hasNonTOTP := len(c.U2F) > 0 || c.WebauthnChallenge != nil

	// Does the current platform support hardware MFA? Adjust accordingly.
	switch {
	case !hasTOTP && !wancli.HasPlatformSupport():
		return nil, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
	case !wancli.HasPlatformSupport():
		// Do not prompt for hardware devices, it won't work.
		hasNonTOTP = false
	}

	var numGoroutines int
	if hasTOTP && hasNonTOTP {
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

	// Fire TOTP goroutine.
	if hasTOTP {
		go func() {
			const kind = "TOTP"
			var msg string
			if !quiet {
				if hasNonTOTP {
					msg = fmt.Sprintf("Tap any %[1]ssecurity key or enter a code from a %[1]sOTP device", promptDevicePrefix, promptDevicePrefix)
				} else {
					msg = fmt.Sprintf("Enter an OTP code from a %sdevice", promptDevicePrefix)
				}
			}
			code, err := promptOTP(ctx, os.Stderr, prompt.Stdin(), msg)
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

	// Fire Webauthn or U2F goroutine.
	origin := proxyAddr
	if !strings.HasPrefix(origin, "https://") {
		origin = "https://" + origin
	}
	switch {
	case c.WebauthnChallenge != nil:
		go func() {
			log.Debugf("WebAuthn: prompting U2F devices with origin %q", origin)
			resp, err := promptWebauthn(ctx, origin, wanlib.CredentialAssertionFromProto(c.WebauthnChallenge))
			respC <- response{kind: "WEBAUTHN", resp: resp, err: err}
		}()
	case len(c.U2F) > 0:
		go func() {
			log.Debugf("prompting U2F devices with facet %q", origin)
			resp, err := promptU2FChallenges(ctx, proxyAddr, c.U2F)
			respC <- response{kind: "U2F", resp: resp, err: err}
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

			// Exiting cancels the context via defer, which makes the remaining
			// goroutines stop.
			return resp.resp, nil
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		}
	}
	return nil, trace.BadParameter(
		"failed to authenticate using all MFA devices, rerun the command with '-d' to see error details for each device")
}

func promptU2FChallenges(ctx context.Context, origin string, challenges []*proto.U2FChallenge) (*proto.MFAAuthenticateResponse, error) {
	u2fChallenges := make([]u2f.AuthenticateChallenge, 0, len(challenges))
	for _, chal := range challenges {
		u2fChallenges = append(u2fChallenges, u2f.AuthenticateChallenge{
			Challenge: chal.Challenge,
			KeyHandle: chal.KeyHandle,
			AppID:     chal.AppID,
		})
	}

	resp, err := promptU2F(ctx, origin, u2fChallenges...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{
		U2F: &proto.U2FResponse{
			KeyHandle:  resp.KeyHandle,
			ClientData: resp.ClientData,
			Signature:  resp.SignatureData,
		},
	}}, nil
}

// MakeAuthenticateChallenge converts proto to JSON format.
func MakeAuthenticateChallenge(protoChal *proto.MFAAuthenticateChallenge) *auth.MFAAuthenticateChallenge {
	chal := &auth.MFAAuthenticateChallenge{
		TOTPChallenge: protoChal.GetTOTP() != nil,
	}

	for _, u2fChal := range protoChal.GetU2F() {
		ch := u2f.AuthenticateChallenge{
			Version:   u2fChal.Version,
			Challenge: u2fChal.Challenge,
			KeyHandle: u2fChal.KeyHandle,
			AppID:     u2fChal.AppID,
		}
		if chal.AuthenticateChallenge == nil {
			chal.AuthenticateChallenge = &ch
		}
		chal.U2FChallenges = append(chal.U2FChallenges, ch)
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
	// U2F contains U2F register challenge.
	U2F *u2f.RegisterChallenge `json:"u2f"`
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

	case *proto.MFARegisterChallenge_U2F:
		return &MFARegisterChallenge{
			U2F: &u2f.RegisterChallenge{
				Version:   protoChal.GetU2F().GetVersion(),
				Challenge: protoChal.GetU2F().GetChallenge(),
				AppID:     protoChal.GetU2F().GetAppID(),
			},
		}

	case *proto.MFARegisterChallenge_Webauthn:
		return &MFARegisterChallenge{
			Webauthn: wanlib.CredentialCreationFromProto(protoChal.GetWebauthn()),
		}
	}

	return nil
}
