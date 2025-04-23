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
	"errors"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/utils/prompt"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client/mfa"
)

func TestCLIPrompt(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name                  string
		stdin                 string
		challenge             *proto.MFAAuthenticateChallenge
		modifyPromptConfig    func(cfg *mfa.CLIPromptConfig)
		expectErr             error
		expectStdOut          string
		expectResp            *proto.MFAAuthenticateResponse
		makeWebauthnLoginFunc func(stdin *prompt.FakeReader) mfa.WebauthnLoginFunc
	}{
		{
			name:         "OK empty challenge",
			expectStdOut: "",
			challenge:    &proto.MFAAuthenticateChallenge{},
			expectResp:   &proto.MFAAuthenticateResponse{},
		},
		{
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
		},
		{
			name:         "OK otp",
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
		},
		{
			name:         "OK sso",
			expectStdOut: "", // sso stdout is handled internally in the SSO ceremony, which is mocked in this test.
			challenge: &proto.MFAAuthenticateChallenge{
				SSOChallenge: &proto.SSOChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_SSO{
					SSO: &proto.SSOResponse{
						RequestId: "request-id",
						Token:     "mfa-token",
					},
				},
			},
		},
		{
			name:         "OK prefer otp when specified",
			expectStdOut: "Enter an OTP code from a device:\n",
			stdin:        "123456",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.PreferOTP = true
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_TOTP{
					TOTP: &proto.TOTPResponse{
						Code: "123456",
					},
				},
			},
		},
		{
			name:         "OK prefer sso when specified",
			expectStdOut: "",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.PreferSSO = true
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_SSO{
					SSO: &proto.SSOResponse{
						RequestId: "request-id",
						Token:     "mfa-token",
					},
				},
			},
		},
		{
			name:         "OK prefer webauthn with authenticator attachment requested",
			expectStdOut: "Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.AuthenticatorAttachment = wancli.AttachmentPlatform
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name: "OK prefer webauthn over sso",
			expectStdOut: "" +
				"Available MFA methods [WEBAUTHN, SSO]. Continuing with WEBAUTHN.\n" +
				"If you wish to perform MFA with another method, specify with flag --mfa-mode=<webauthn,sso>.\n\n" +
				"Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name: "OK prefer webauthn+otp over sso",
			expectStdOut: "" +
				"Available MFA methods [WEBAUTHN, SSO, OTP]. Continuing with WEBAUTHN and OTP.\n" +
				"If you wish to perform MFA with another method, specify with flag --mfa-mode=<webauthn,sso,otp>.\n\n" +
				"Tap any security key or enter a code from a OTP device\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.AllowStdinHijack = true
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name: "OK prefer sso over otp",
			expectStdOut: "" +
				"Available MFA methods [SSO, OTP]. Continuing with SSO.\n" +
				"If you wish to perform MFA with another method, specify with flag --mfa-mode=<sso,otp>.\n\n",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP:         &proto.TOTPChallenge{},
				SSOChallenge: &proto.SSOChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_SSO{
					SSO: &proto.SSOResponse{
						RequestId: "request-id",
						Token:     "mfa-token",
					},
				},
			},
		},
		{
			name: "OK prefer webauthn over otp when stdin hijack disallowed",
			expectStdOut: "" +
				"Available MFA methods [WEBAUTHN, OTP]. Continuing with WEBAUTHN.\n" +
				"If you wish to perform MFA with another method, specify with flag --mfa-mode=<webauthn,otp>.\n\n" +
				"Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name: "OK webauthn or otp with stdin hijack allowed, choose webauthn",
			expectStdOut: "" +
				"Available MFA methods [WEBAUTHN, SSO, OTP]. Continuing with WEBAUTHN and OTP.\n" +
				"If you wish to perform MFA with another method, specify with flag --mfa-mode=<webauthn,sso,otp>.\n\n" +
				"Tap any security key or enter a code from a OTP device\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.AllowStdinHijack = true
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name: "OK webauthn or otp with stdin hijack allowed, choose otp",
			expectStdOut: "" +
				"Available MFA methods [WEBAUTHN, SSO, OTP]. Continuing with WEBAUTHN and OTP.\n" +
				"If you wish to perform MFA with another method, specify with flag --mfa-mode=<webauthn,sso,otp>.\n\n" +
				"Tap any security key or enter a code from a OTP device\n",
			stdin: "123456",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
				SSOChallenge:      &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.AllowStdinHijack = true
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_TOTP{
					TOTP: &proto.TOTPResponse{
						Code: "123456",
					},
				},
			},
		},
		{
			name:         "NOK no webauthn response",
			expectStdOut: "Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			expectErr: context.DeadlineExceeded,
		},
		{
			name:         "NOK no sso response",
			expectStdOut: "",
			challenge: &proto.MFAAuthenticateChallenge{
				SSOChallenge: &proto.SSOChallenge{},
			},
			expectErr: context.DeadlineExceeded,
		},
		{
			name:         "NOK no otp response",
			expectStdOut: "Enter an OTP code from a device:\n",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP: &proto.TOTPChallenge{},
			},
			expectErr: context.DeadlineExceeded,
		},
		{
			name:         "NOK no webauthn or otp response",
			expectStdOut: "Tap any security key or enter a code from a OTP device\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
				TOTP:              &proto.TOTPChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.AllowStdinHijack = true
			},
			expectErr: context.DeadlineExceeded,
		},
		{
			name: "OK otp and webauthn with PIN",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP:              &proto.TOTPChallenge{},
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.AllowStdinHijack = true
			},
			expectStdOut: `Tap any security key or enter a code from a OTP device
Detected security key tap
Enter your security key PIN:
`,
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{
						RawId: []byte{1, 2, 3, 4, 5},
					},
				},
			},
			makeWebauthnLoginFunc: func(stdin *prompt.FakeReader) mfa.WebauthnLoginFunc {
				return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
					ack, err := prompt.PromptTouch()
					if err != nil {
						return nil, "", trace.Wrap(err)
					}

					// Ack first (so the OTP goroutine stops)...
					if err := ack(); err != nil {
						return nil, "", trace.Wrap(err)
					}

					// ...then send the PIN to stdin...
					const pin = "1234"
					stdin.AddString(pin)

					// ...then prompt for the PIN.
					switch got, err := prompt.PromptPIN(); {
					case err != nil:
						return nil, "", trace.Wrap(err)
					case got != pin:
						return nil, "", errors.New("invalid PIN")
					}

					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: &webauthnpb.CredentialAssertionResponse{
								RawId: []byte{1, 2, 3, 4, 5},
							},
						},
					}, "", nil
				}
			},
		},
		{
			name: "OK webauthn with PIN",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP:              nil, // no TOTP challenge
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			stdin: "1234",
			expectStdOut: `Tap any security key
Detected security key tap
Enter your security key PIN:
`,
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{
						RawId: []byte{1, 2, 3, 4, 5},
					},
				},
			},
			makeWebauthnLoginFunc: func(_ *prompt.FakeReader) mfa.WebauthnLoginFunc {
				return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
					ack, err := prompt.PromptTouch()
					if err != nil {
						return nil, "", trace.Wrap(err)
					}
					if err := ack(); err != nil {
						return nil, "", trace.Wrap(err)
					}

					switch got, err := prompt.PromptPIN(); {
					case err != nil:
						return nil, "", trace.Wrap(err)
					case got != "1234":
						return nil, "", errors.New("invalid PIN")
					}

					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: &webauthnpb.CredentialAssertionResponse{
								RawId: []byte{1, 2, 3, 4, 5},
							},
						},
					}, "", nil
				}
			},
		},
		{
			name: "NOK webauthn and SSO not supported",
			challenge: &proto.MFAAuthenticateChallenge{
				SSOChallenge:      &proto.SSOChallenge{},
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.WebauthnSupported = false
				cfg.SSOMFACeremony = nil
			},
			expectErr: trace.BadParameter("client does not support any available MFA methods [WEBAUTHN, SSO], see debug logs for details"),
		},
		{
			name: "NOK otp with per-session MFA",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP: &proto.TOTPChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.Extensions = &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
				}
			},
			expectErr: trace.AccessDenied("only WebAuthn and SSO MFA methods are supported with per-session MFA"),
		},
		{
			name: "NOK prefer otp with per-session MFA",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP: &proto.TOTPChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.Extensions = &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
				}
				cfg.PreferOTP = true
			},
			expectErr: trace.AccessDenied("only WebAuthn and SSO MFA methods are supported with per-session MFA, can not specify --mfa-mode=otp"),
		},
		{
			name:         "OK webauthn or otp with stdin hijack and per-session MFA, no choice presented",
			expectStdOut: "Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				TOTP:              &proto.TOTPChallenge{},
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.Extensions = &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
				}
				cfg.AllowStdinHijack = true
			},
			// expect to go down normal webauthn path instead of promptWebauthnAndOTP
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name:         "OK webauthn with per-session MFA",
			expectStdOut: "Tap any security key\n",
			challenge: &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthnpb.CredentialAssertion{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.Extensions = &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
				}
				cfg.AllowStdinHijack = true
			},
			// expect to go down normal webauthn path instead of promptWebauthnAndOTP
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &webauthnpb.CredentialAssertionResponse{},
				},
			},
		},
		{
			name:         "OK sso with per-session MFA",
			expectStdOut: "", // sso stdout is handled internally in the SSO ceremony, which is mocked in this test.
			challenge: &proto.MFAAuthenticateChallenge{
				SSOChallenge: &proto.SSOChallenge{},
			},
			modifyPromptConfig: func(cfg *mfa.CLIPromptConfig) {
				cfg.Extensions = &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
				}
				cfg.AllowStdinHijack = true
			},
			expectResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_SSO{
					SSO: &proto.SSOResponse{
						RequestId: "request-id",
						Token:     "mfa-token",
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			stdin := prompt.NewFakeReader()
			if tc.stdin != "" {
				stdin.AddString(tc.stdin)
			}

			cfg := mfa.NewPromptConfig("proxy.example.com")
			cfg.WebauthnSupported = true
			if tc.makeWebauthnLoginFunc != nil {
				cfg.WebauthnLoginFunc = tc.makeWebauthnLoginFunc(stdin)
			} else {
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
			}

			cfg.SSOMFACeremony = &mockSSOMFACeremony{
				mfaResp: tc.expectResp,
			}

			buffer := make([]byte, 0, 100)
			out := bytes.NewBuffer(buffer)

			cliPromptConfig := &mfa.CLIPromptConfig{
				PromptConfig: *cfg,
				Writer:       out,
				StdinFunc: func() prompt.StdinReader {
					return stdin
				},
			}

			if tc.modifyPromptConfig != nil {
				tc.modifyPromptConfig(cliPromptConfig)
			}

			resp, err := mfa.NewCLIPrompt(cliPromptConfig).Run(ctx, tc.challenge)
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

type mockSSOMFACeremony struct {
	mfaResp *proto.MFAAuthenticateResponse
}

func (m *mockSSOMFACeremony) GetClientCallbackURL() string {
	return ""
}

// Run the SSO MFA ceremony.
func (m *mockSSOMFACeremony) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if m.mfaResp == nil {
		return nil, context.DeadlineExceeded
	}
	if m.mfaResp.GetSSO() == nil {
		return nil, trace.BadParameter("expected an SSO response but got %T", m.mfaResp.Response)
	}
	return m.mfaResp, nil
}

func (m *mockSSOMFACeremony) Close() {}
