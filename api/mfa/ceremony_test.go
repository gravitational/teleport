/*
Copyright 2024 Gravitational, Inc.

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

package mfa_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	webauthn "github.com/gravitational/teleport/api/types/webauthn"
)

type mockPrompt struct {
	authResponse         *proto.MFAAuthenticateResponse
	regResult            *mfa.RegistrationResult
	askedToRegister      bool
	notifiedAboutSuccess bool
}

func (m *mockPrompt) Run(
	ctx context.Context, chal *proto.MFAAuthenticateChallenge,
) (*proto.MFAAuthenticateResponse, error) {
	return m.authResponse, nil
}

func (m *mockPrompt) AskRegister(
	ctx context.Context, config mfa.RegistrationPromptConfig,
) (*mfa.RegistrationPromptConfig, error) {
	m.askedToRegister = true
	config.RegistrationCeremonyConfig = mfa.RegistrationCeremonyConfig{
		DeviceType: mfa.MFADeviceTypeWebauthn,
		DeviceName: "new-device",
	}
	return &config, nil
}

func (m *mockPrompt) RunRegister(
	ctx context.Context, config mfa.RegistrationPromptConfig, challenge *proto.MFARegisterChallenge,
) (*mfa.RegistrationResult, error) {
	return m.regResult, nil
}

func (m *mockPrompt) NotifyRegistrationSuccess(
	ctx context.Context, config mfa.RegistrationPromptConfig,
) error {
	m.notifiedAboutSuccess = true
	return nil
}

func TestMFACeremony(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testOTPChallenge := &proto.MFAAuthenticateChallenge{
		TOTP: &proto.TOTPChallenge{},
	}
	testOTPResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: "otp-test-code",
			},
		},
	}

	testWebauthnChallenge := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: &webauthn.CredentialAssertion{
			PublicKey: &webauthn.PublicKeyCredentialRequestOptions{
				Challenge: []byte("webauthn-challenge"),
			},
		},
	}
	testWebauthnResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: &webauthn.CredentialAssertionResponse{
				Type:  "public-key",
				RawId: []byte("raw-id"),
				Response: &webauthn.AuthenticatorAssertionResponse{
					ClientDataJson:    []byte("client data json"),
					AuthenticatorData: []byte("authenticator data"),
					Signature:         []byte("signature"),
					UserHandle:        []byte("user handle"),
				},
			},
		},
	}

	for _, tt := range []struct {
		name                   string
		ceremony               *mfa.Ceremony
		scope                  mfav1.ChallengeScope
		assertCeremonyResponse func(*testing.T, *proto.MFAAuthenticateResponse, error, ...interface{})
	}{
		{
			name: "OK OTP ceremony success prompt",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testOTPChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return testOTPResponse, nil
					})
				},
			},
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.NoError(t, err)
				assert.Equal(t, testOTPResponse, mr)
			},
		}, {
			name: "OK WebAuthn ceremony success prompt",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testWebauthnChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return testWebauthnResponse, nil
					})
				},
			},
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.NoError(t, err)
				assert.Equal(t, testWebauthnResponse, mr)
			},
		}, {
			name: "OK ceremony not required",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return &proto.MFAAuthenticateChallenge{
						MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
					}, nil
				},
				PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, trace.BadParameter("expected mfa not required")
					})
				},
			},
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.Error(t, err, mfa.ErrMFANotRequired)
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK create challenge fail",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return nil, errors.New("create authenticate challenge failure")
				},
				PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, trace.BadParameter("expected challenge failure")
					})
				},
			},
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "create authenticate challenge failure")
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK prompt mfa fail",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testOTPChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, errors.New("prompt mfa failure")
					})
				},
			},
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "prompt mfa failure")
				assert.Nil(t, mr)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: tt.scope,
				},
				MFARequiredCheck: &proto.IsMFARequiredRequest{},
			})
			tt.assertCeremonyResponse(t, resp, err)
		})
	}
}

func TestMFACeremony_SSO(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testMFAChallenge := &proto.MFAAuthenticateChallenge{
		SSOChallenge: &proto.SSOChallenge{
			RedirectUrl: "redirect",
			RequestId:   "request-id",
		},
	}
	testMFAResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_SSO{
			SSO: &proto.SSOResponse{
				Token:     "token",
				RequestId: "request-id",
			},
		},
	}

	ssoMFACeremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			return testMFAChallenge, nil
		},
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			cfg := new(mfa.PromptConfig)
			for _, opt := range opts {
				opt(cfg)
			}

			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				if cfg.CallbackCeremony == nil {
					return nil, trace.BadParameter("expected mfa ceremony")
				}

				return cfg.CallbackCeremony.Run(ctx, chal)
			})
		},
		MFACeremonyConstructor: func(ctx context.Context) (mfa.CallbackCeremony, error) {
			return &mockMFACeremony{
				clientCallbackURL: "client-redirect",
				prompt: func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
					return testMFAResponse, nil
				},
			}, nil
		},
	}

	resp, err := ssoMFACeremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
		},
		MFARequiredCheck: &proto.IsMFARequiredRequest{},
	})
	require.NoError(t, err)
	require.Equal(t, testMFAResponse, resp)
}

func TestMFACeremony_BrowserMFA(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	expectedCallbackURL := "http://localhost:12345/?secret=X"

	testMFAChallenge := &proto.MFAAuthenticateChallenge{
		BrowserMFAChallenge: &proto.BrowserMFAChallenge{
			RequestId: "request-id",
		},
	}
	testMFAResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Browser{
			Browser: &proto.BrowserMFAResponse{
				RequestId: "request-id",
			},
		},
	}

	browserMFACeremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			require.NotNil(t, req)
			require.Equal(t, expectedCallbackURL, req.BrowserMFATSHRedirectURL)
			return testMFAChallenge, nil
		},
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			cfg := new(mfa.PromptConfig)
			for _, opt := range opts {
				opt(cfg)
			}

			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				if cfg.CallbackCeremony == nil {
					return nil, trace.BadParameter("expected mfa ceremony")
				}

				return cfg.CallbackCeremony.Run(ctx, chal)
			})
		},
		MFACeremonyConstructor: func(ctx context.Context) (mfa.CallbackCeremony, error) {
			return &mockMFACeremony{
				clientCallbackURL: expectedCallbackURL,
				prompt: func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
					return testMFAResponse, nil
				},
			}, nil
		},
		CreateRegisterChallenge: func(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
			return &proto.MFARegisterChallenge{}, nil
		},
		AddMFADevice: func(ctx context.Context, req *proto.MFARegisterResponse, config mfa.RegistrationCeremonyConfig) error {
			return nil
		},
	}

	testCases := []struct {
		name  string
		scope mfav1.ChallengeScope
	}{
		{
			name:  "login",
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		},
		{
			name:  "admin action",
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := browserMFACeremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: tc.scope,
				},
				MFARequiredCheck: &proto.IsMFARequiredRequest{},
			})
			require.NoError(t, err)
			require.Equal(t, testMFAResponse, resp)
		})
	}
}

func TestMFACeremony_NilRequest(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	expectedResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: &webauthn.CredentialAssertionResponse{},
		},
	}

	ceremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			return &proto.MFAAuthenticateChallenge{
				WebauthnChallenge: &webauthn.CredentialAssertion{},
			}, nil
		},
		PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				return expectedResponse, nil
			})
		},
	}

	resp, err := ceremony.Run(ctx, nil)

	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, resp)
}

func TestMFACeremony_Registration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	testCases := []struct {
		scope  mfav1.ChallengeScope
		assert func(t *testing.T, err error, mp *mockPrompt, callbacks *mockRegistrationCallbacks, devicesAdded []string)
	}{
		{
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
			assert: func(t *testing.T, err error, mp *mockPrompt, callbacks *mockRegistrationCallbacks, devicesAdded []string) {
				require.NoError(t, err)
				assert.True(t, mp.askedToRegister)
				assert.True(t, mp.notifiedAboutSuccess)
				assert.False(t, callbacks.rolledBack)
				assert.True(t, callbacks.confirmed)
				assert.Equal(t, []string{"new-device"}, devicesAdded)
			},
		}, {
			scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			assert: func(t *testing.T, err error, mp *mockPrompt, callbacks *mockRegistrationCallbacks, devicesAdded []string) {
				require.NoError(t, err)
				assert.False(t, mp.askedToRegister)
				assert.False(t, mp.notifiedAboutSuccess)
				assert.False(t, callbacks.rolledBack)
				assert.False(t, callbacks.confirmed)
				assert.Empty(t, devicesAdded)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.scope.String(), func(t *testing.T) {
			callbacks := mockRegistrationCallbacks{}
			mp := mockPrompt{
				authResponse: &proto.MFAAuthenticateResponse{},
				regResult: &mfa.RegistrationResult{
					Callbacks: &callbacks,
					Response: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: &webauthn.CredentialCreationResponse{
								Type:  "public-key",
								RawId: []byte("some-id"),
							},
						},
					},
				},
			}

			var devicesAdded []string
			ceremony := &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					chal := &proto.MFAAuthenticateChallenge{
						MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
					}
					if len(devicesAdded) > 0 {
						chal.WebauthnChallenge = &webauthn.CredentialAssertion{}
					}
					return chal, nil
				},
				CreateRegisterChallenge: func(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
					return &proto.MFARegisterChallenge{}, nil
				},
				AddMFADevice: func(ctx context.Context, req *proto.MFARegisterResponse, config mfa.RegistrationCeremonyConfig) error {
					devicesAdded = append(devicesAdded, config.DeviceName)
					return nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return &mp
				},
			}

			_, err := ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: tc.scope,
				},
				MFARequiredCheck: &proto.IsMFARequiredRequest{},
			})
			require.NoError(t, err)
			tc.assert(t, err, &mp, &callbacks, devicesAdded)
		})
	}
}

type mockRegistrationCallbacks struct {
	rolledBack bool
	confirmed  bool
}

func (m *mockRegistrationCallbacks) Rollback() error {
	m.rolledBack = true
	return nil
}

func (m *mockRegistrationCallbacks) Confirm() error {
	m.confirmed = true
	return nil
}

type mockMFACeremony struct {
	clientCallbackURL string
	proxyAddress      string
	prompt            mfa.PromptFunc
	closeFunc         func()
}

// GetClientCallbackURL returns the client callback URL.
func (m *mockMFACeremony) GetClientCallbackURL() string {
	return m.clientCallbackURL
}

func (m *mockMFACeremony) GetProxyAddress() string {
	return m.proxyAddress
}

// Run the SSO MFA ceremony.
func (m *mockMFACeremony) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return m.prompt(ctx, chal)
}

func (m *mockMFACeremony) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}
