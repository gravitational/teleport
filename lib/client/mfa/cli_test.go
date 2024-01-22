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

package mfa_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client/mfa"
)

func TestCLIPrompt(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name         string
		stdin        string
		challenge    *proto.MFAAuthenticateChallenge
		expectErr    error
		expectStdOut string
		expectResp   *proto.MFAAuthenticateResponse
	}{
		{
			name:         "OK empty challenge",
			expectStdOut: "",
			challenge:    &proto.MFAAuthenticateChallenge{},
			expectResp:   &proto.MFAAuthenticateResponse{},
		}, {
			name:         "OK webauthn",
			expectStdOut: "Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		}, {
			name:         "OK totp",
			expectStdOut: "Enter an OTP code from a device:\n",
			stdin:        "123456",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP: &proto.TOTPChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_TOTP{
					TOTP: &proto.TOTPResponse{
						Code: "123456",
					},
				},
			},
		}, {
			name:         "OK webauthn or totp choose webauthn",
			expectStdOut: "Tap any security key or enter a code from a OTP device\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		}, {
			name:         "OK webauthn or totp choose totp",
			expectStdOut: "Tap any security key or enter a code from a OTP device\n",
			stdin:        "123456",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_TOTP{
					TOTP: &proto.TOTPResponse{
						Code: "123456",
					},
				},
			},
		}, {
			name:         "NOK no webauthn response",
			expectStdOut: "Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			expectErr: context.DeadlineExceeded,
		}, {
			name:         "NOK no totp response",
			expectStdOut: "Enter an OTP code from a device:\n",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP: &proto.TOTPChallenge{},
			},
			expectErr: context.DeadlineExceeded,
		}, {
			name:         "NOK no webauthn or totp response",
			expectStdOut: "Tap any security key or enter a code from a OTP device\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
			},
			expectErr: context.DeadlineExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			oldStdin := prompt.Stdin()
			t.Cleanup(func() { prompt.SetStdin(oldStdin) })

			stdin := prompt.NewFakeReader()
			if tc.stdin != "" {
				stdin.AddString(tc.stdin)
			}
			prompt.SetStdin(stdin)

			cfg := mfa.NewPromptConfig("proxy.example.com")
			cfg.AllowStdinHijack = true
			cfg.WebauthnSupported = true
			cfg.WebauthnLoginFunc = func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
				if _, err := prompt.PromptTouch(); err != nil {
					return nil, "", trace.Wrap(err)
				}

				if tc.expectResp.GetWebauthn() == nil {
					<-ctx.Done()
					return nil, "", trace.Wrap(ctx.Err())
				}

				return tc.expectResp, "", nil
			}

			buffer := make([]byte, 0, 100)
			out := bytes.NewBuffer(buffer)

			prompt := mfa.NewCLIPrompt(cfg, out)
			resp, err := prompt.Run(ctx, tc.challenge)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectResp, resp)
			require.Equal(t, tc.expectStdOut, out.String())
		})
	}
}
