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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/utils/prompt"
)

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
//
// If promptDevicePrefix is set, it will be printed in prompts before "security
// key" or "device". This is used to emphasize between different kinds of
// devices, like registered vs new.
func PromptMFAChallenge(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge, promptDevicePrefix string) (*proto.MFAAuthenticateResponse, error) {
	switch {
	// No challenge.
	case c.TOTP == nil && len(c.U2F) == 0:
		return &proto.MFAAuthenticateResponse{}, nil
	// TOTP only.
	case c.TOTP != nil && len(c.U2F) == 0:
		totpCode, err := prompt.Input(os.Stderr, os.Stdin, fmt.Sprintf("Enter an OTP code from a %sdevice", promptDevicePrefix))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: totpCode},
		}}, nil
	// U2F only.
	case c.TOTP == nil && len(c.U2F) > 0:
		fmt.Fprintf(os.Stderr, "Tap any %ssecurity key\n", promptDevicePrefix)

		return promptU2FChallenges(ctx, proxyAddr, c.U2F)
	// Both TOTP and U2F.
	case c.TOTP != nil && len(c.U2F) > 0:
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		type response struct {
			kind string
			resp *proto.MFAAuthenticateResponse
			err  error
		}
		resCh := make(chan response, 1)

		go func() {
			resp, err := promptU2FChallenges(ctx, proxyAddr, c.U2F)
			select {
			case resCh <- response{kind: "U2F", resp: resp, err: err}:
			case <-ctx.Done():
			}
		}()

		go func() {
			totpCode, err := prompt.Input(os.Stderr, os.Stdin, fmt.Sprintf("Tap any %[1]ssecurity key or enter a code from a %[1]sOTP device", promptDevicePrefix, promptDevicePrefix))
			res := response{kind: "TOTP", err: err}
			if err == nil {
				res.resp = &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
					TOTP: &proto.TOTPResponse{Code: totpCode},
				}}
			}

			select {
			case resCh <- res:
			case <-ctx.Done():
			}
		}()

		for i := 0; i < 2; i++ {
			select {
			case res := <-resCh:
				if res.err != nil {
					log.WithError(res.err).Debugf("%s authentication failed", res.kind)
					continue
				}

				// Print a newline after the TOTP prompt, so that any future
				// output doesn't print on the prompt line.
				fmt.Fprintln(os.Stderr)

				return res.resp, nil
			case <-ctx.Done():
				return nil, trace.Wrap(ctx.Err())
			}
		}
		return nil, trace.Errorf("failed to authenticate using all U2F and TOTP devices, rerun the command with '-d' to see error details for each device")
	default:
		return nil, trace.BadParameter("bug: non-exhaustive switch in promptMFAChallenge")
	}
}

func promptU2FChallenges(ctx context.Context, proxyAddr string, challenges []*proto.U2FChallenge) (*proto.MFAAuthenticateResponse, error) {
	facet := proxyAddr
	if !strings.HasPrefix(proxyAddr, "https://") {
		facet = "https://" + facet
	}
	u2fChallenges := make([]u2f.AuthenticateChallenge, 0, len(challenges))
	for _, chal := range challenges {
		u2fChallenges = append(u2fChallenges, u2f.AuthenticateChallenge{
			Challenge: chal.Challenge,
			KeyHandle: chal.KeyHandle,
			AppID:     chal.AppID,
		})
	}

	log.Debugf("prompting U2F devices with facet %q", facet)
	resp, err := u2f.AuthenticateSignChallenge(ctx, facet, u2fChallenges...)
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
