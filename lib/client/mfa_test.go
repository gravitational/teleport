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

package client_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"

	"github.com/gravitational/teleport/api/client/proto"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client/mfa"
)

// TestPromptMFAChallenge_usingNonRegisteredDevice tests a specific MFA scenario
// where the user picks a non-registered security key.
// See api_login_test.go and/or TeleportClient tests for more general
// authentication tests.
func TestPromptMFAChallenge_usingNonRegisteredDevice(t *testing.T) {
	oldStdin := prompt.Stdin()
	t.Cleanup(func() {
		prompt.SetStdin(oldStdin)
	})

	const proxyAddr = "example.com"
	ctx := context.Background()

	// The Webauthn challenge below looks like a typical MFA challenge.
	challengeWebauthnOnly := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: &wanpb.CredentialAssertion{
			PublicKey: &wanpb.PublicKeyCredentialRequestOptions{
				Challenge: []byte{1, 2, 3, 4, 5}, // arbitrary
				RpId:      "example.com",
				AllowCredentials: []*wanpb.CredentialDescriptor{
					{
						Type: string(protocol.PublicKeyCredentialType),
						Id:   []byte{5, 5, 5, 5, 5}, // arbitrary
					},
				},
				UserVerification: string(protocol.VerificationDiscouraged),
			},
		},
	}

	challengeWebauthnOTP := &proto.MFAAuthenticateChallenge{
		TOTP:              &proto.TOTPChallenge{}, // non-nil enables OTP prompt
		WebauthnChallenge: challengeWebauthnOnly.WebauthnChallenge,
	}

	tests := []struct {
		name            string
		challenge       *proto.MFAAuthenticateChallenge
		customizePrompt func(p *mfa.CLIPromptConfig)
	}{
		{
			name:      "webauthn only",
			challenge: challengeWebauthnOnly,
		},
		{
			name:      "webauthn and OTP",
			challenge: challengeWebauthnOTP,
			customizePrompt: func(p *mfa.CLIPromptConfig) {
				p.AllowStdinHijack = true // required for OTP+WebAuthn prompt.
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test := test
			t.Parallel()

			// Set a timeout so the test won't block forever.
			// We don't expect to hit the timeout for any of the test cases.
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			// Prompt never has any input.
			prompt.SetStdin(prompt.NewFakeReader().AddReply(func(ctx context.Context) (string, error) {
				<-ctx.Done()
				return "", ctx.Err()
			}))

			promptConfig := mfa.NewPromptConfig(proxyAddr)
			promptConfig.WebauthnSupported = true
			promptConfig.WebauthnLoginFunc = func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
				return nil, "", wancli.ErrUsingNonRegisteredDevice
			}

			cliConfig := &mfa.CLIPromptConfig{
				PromptConfig: *promptConfig,
			}

			if test.customizePrompt != nil {
				test.customizePrompt(cliConfig)
			}

			_, err := mfa.NewCLIPrompt(cliConfig).Run(ctx, test.challenge)
			if !errors.Is(err, wancli.ErrUsingNonRegisteredDevice) {
				t.Errorf("PromptMFAChallenge returned err=%q, want %q", err, wancli.ErrUsingNonRegisteredDevice)
			}
		})
	}
}
