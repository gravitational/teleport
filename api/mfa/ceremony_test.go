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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
)

func TestMFACeremony(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testMFAChallenge := &proto.MFAAuthenticateChallenge{
		TOTP: &proto.TOTPChallenge{},
	}
	testMFAResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: "otp-test-code",
			},
		},
	}

	for _, tt := range []struct {
		name                   string
		ceremony               *mfa.Ceremony
		assertCeremonyResponse func(*testing.T, *proto.MFAAuthenticateResponse, error, ...interface{})
	}{
		{
			name: "OK ceremony success prompt",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testMFAChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return testMFAResponse, nil
					})
				},
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.NoError(t, err)
				assert.Equal(t, testMFAResponse, mr)
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
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "create authenticate challenge failure")
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK prompt mfa fail",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testMFAChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, errors.New("prompt mfa failure")
					})
				},
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "prompt mfa failure")
				assert.Nil(t, mr)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
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
	}

	resp, err := browserMFACeremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		},
		MFARequiredCheck: &proto.IsMFARequiredRequest{},
	})
	require.NoError(t, err)
	require.Equal(t, testMFAResponse, resp)
}

const (
	mockTargetCluster       = "root"
	mockRedirectURL         = "https://example.com/redirect"
	mockCallbackURL         = "https://example.com/callback"
	mockProxyAddress        = "proxy.example.com:443"
	mockSSORequestID        = "sso-request-id"
	mockSSOToken            = "sso-token"
	mockChallengeName       = "challenge"
	mockWebauthnChallenge   = "challenge"
	mockCredentialID        = "credential-id"
	mockBrowserRequestID    = "browser-request-id"
	mockBrowserCredentialID = "browser-credential-id"
)

func TestNewSessionBoundCeremony(t *testing.T) {
	t.Parallel()

	ceremony, err := mfa.NewSessionBoundCeremony(newSessionBindingConfig())
	require.NoError(t, err)
	require.NotEmpty(t, ceremony)
}

func TestNewSessionBoundCeremonyErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		buildConfig func() mfa.SessionBoundCeremonyConfig
		wantErr     error
	}{
		{
			name: "missing payload",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				return mfa.SessionBoundCeremonyConfig{
					CreateSessionChallenge:   newValidSessionBindingConfig().CreateSessionChallenge,
					ValidateSessionChallenge: newValidSessionBindingConfig().ValidateSessionChallenge,
					PromptConstructor:        newValidSessionBindingConfig().PromptConstructor,
					TargetCluster:            mockTargetCluster,
				}
			},
			wantErr: trace.BadParameter("config.Payload must not be nil"),
		},
		{
			name: "missing create session challenge func",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = nil

				return config
			},
			wantErr: trace.BadParameter("config.CreateSessionChallenge must not be nil"),
		},
		{
			name: "missing target cluster",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.TargetCluster = ""

				return config
			},
			wantErr: trace.BadParameter("config.TargetCluster must not be empty"),
		},
		{
			name: "missing prompt constructor",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.PromptConstructor = nil

				return config
			},
			wantErr: trace.BadParameter("config.PromptConstructor must not be nil"),
		},
		{
			name: "missing validate session challenge",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.ValidateSessionChallenge = nil

				return config
			},
			wantErr: trace.BadParameter("config.ValidateSessionChallenge must not be nil"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ceremony, err := mfa.NewSessionBoundCeremony(test.buildConfig())
			require.ErrorIs(t, err, test.wantErr)
			require.Empty(t, ceremony)
		})
	}
}

func TestSessionBoundCeremonyRun(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		createResp       *mfav1.CreateSessionChallengeResponse
		promptResp       *proto.MFAAuthenticateResponse
		wantPromptChal   *proto.MFAAuthenticateChallenge
		wantValidateResp *mfav1.AuthenticateResponse
		callbackCeremony mfa.CallbackCeremony
	}{
		{
			name: "webauthn response",
			createResp: func() *mfav1.CreateSessionChallengeResponse {
				resp := newSessionBindingCreateResp("challenge-webauthn")
				resp.MfaChallenge.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

				return resp
			}(),
			promptResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: newWebauthnResponse(mockCredentialID),
				},
			},
			wantPromptChal: func() *proto.MFAAuthenticateChallenge {
				chal := newProtoMFAChallenge()
				chal.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

				return chal
			}(),
			wantValidateResp: &mfav1.AuthenticateResponse{
				Name: "challenge-webauthn",
				Response: &mfav1.AuthenticateResponse_Webauthn{
					Webauthn: newWebauthnResponse(mockCredentialID),
				},
			},
		},
		{
			name: "sso response",
			createResp: func() *mfav1.CreateSessionChallengeResponse {
				resp := newSessionBindingCreateResp("challenge-sso")
				resp.MfaChallenge.SsoChallenge = newMFASSOChallenge()

				return resp
			}(),
			promptResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_SSO{
					SSO: newProtoSSOResponse(mockSSOToken),
				},
			},
			wantPromptChal: func() *proto.MFAAuthenticateChallenge {
				chal := newProtoMFAChallenge()
				chal.SSOChallenge = newProtoSSOChallenge()

				return chal
			}(),
			wantValidateResp: &mfav1.AuthenticateResponse{
				Name: "challenge-sso",
				Response: &mfav1.AuthenticateResponse_Sso{
					Sso: &mfav1.SSOChallengeResponse{
						RequestId: mockSSORequestID,
						Token:     mockSSOToken,
					},
				},
			},
			callbackCeremony: &mockMFACeremony{
				clientCallbackURL: mockCallbackURL,
				proxyAddress:      mockProxyAddress,
			},
		},
		{
			name: "browser response",
			createResp: func() *mfav1.CreateSessionChallengeResponse {
				resp := newSessionBindingCreateResp("challenge-browser")
				resp.MfaChallenge.BrowserChallenge = &mfav1.BrowserMFAChallenge{
					RequestId: mockBrowserRequestID,
				}

				return resp
			}(),
			promptResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Browser{
					Browser: newProtoBrowserResponse(mockBrowserRequestID, newWebauthnResponse(mockBrowserCredentialID)),
				},
			},
			wantPromptChal: func() *proto.MFAAuthenticateChallenge {
				chal := newProtoMFAChallenge()
				chal.BrowserMFAChallenge = &proto.BrowserMFAChallenge{
					RequestId: mockBrowserRequestID,
				}

				return chal
			}(),
			wantValidateResp: &mfav1.AuthenticateResponse{
				Name: "challenge-browser",
				Response: &mfav1.AuthenticateResponse_Browser{
					Browser: newMFABrowserResponse(mockBrowserRequestID, newWebauthnResponse(mockBrowserCredentialID)),
				},
			},
			callbackCeremony: &mockMFACeremony{
				clientCallbackURL: mockCallbackURL,
				proxyAddress:      mockProxyAddress,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := newSessionBindingConfig()

			ceremony, err := mfa.NewSessionBoundCeremony(mfa.SessionBoundCeremonyConfig{
				CreateSessionChallenge: func(_ context.Context, req *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					require.Empty(t, cmp.Diff(config.Payload, req.GetPayload(), protocmp.Transform()))
					require.Equal(t, config.TargetCluster, req.GetTargetCluster())

					if test.callbackCeremony == nil {
						require.Empty(t, req.GetSsoClientRedirectUrl())
						require.Empty(t, req.GetProxyAddressForSso())
						require.Empty(t, req.GetBrowserMfaTshRedirectUrl())
					} else {
						require.Equal(t, test.callbackCeremony.GetClientCallbackURL(), req.GetSsoClientRedirectUrl())
						require.Equal(t, test.callbackCeremony.GetProxyAddress(), req.GetProxyAddressForSso())
						require.Equal(t, test.callbackCeremony.GetClientCallbackURL(), req.GetBrowserMfaTshRedirectUrl())
					}

					return test.createResp, nil
				},
				PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
					cfg := new(mfa.PromptConfig)
					for _, opt := range opts {
						opt(cfg)
					}

					return mfa.PromptFunc(
						func(_ context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
							require.Empty(t, cmp.Diff(test.wantPromptChal, chal, protocmp.Transform()))

							if test.callbackCeremony == nil {
								require.Nil(t, cfg.CallbackCeremony)
							} else {
								require.NotNil(t, cfg.CallbackCeremony)
							}

							return test.promptResp, nil
						},
					)
				},
				ValidateSessionChallenge: func(_ context.Context, req *mfav1.ValidateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.ValidateSessionChallengeResponse, error) {
					require.Empty(t, cmp.Diff(test.wantValidateResp, req.GetMfaResponse(), protocmp.Transform()))

					return &mfav1.ValidateSessionChallengeResponse{}, nil
				},
				CallbackCeremony: test.callbackCeremony,
				Payload:          config.Payload,
				TargetCluster:    config.TargetCluster,
			})
			require.NoError(t, err)

			gotName, err := ceremony.Run(t.Context())
			require.NoError(t, err)
			require.Equal(t, test.createResp.MfaChallenge.GetName(), gotName, "expected challenge name to be returned")
		})
	}
}

func TestSessionBoundCeremonyRun_CallbackCeremony(t *testing.T) {
	t.Parallel()

	mockCallbackURL := "https://example.com/session-bound-callback"
	mockCallbackProxyAddress := "proxy.example.com:3080"
	wantResp := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_SSO{
			SSO: newProtoSSOResponse(mockSSOToken),
		},
	}

	config := newSessionBindingConfig()
	config.CallbackCeremony = &mockMFACeremony{
		clientCallbackURL: mockCallbackURL,
		proxyAddress:      mockCallbackProxyAddress,
		prompt: func(_ context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
			require.NotNil(t, chal.GetSSOChallenge())

			return wantResp, nil
		},
	}
	config.CreateSessionChallenge = func(_ context.Context, req *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
		require.Equal(t, mockCallbackURL, req.GetSsoClientRedirectUrl())
		require.Equal(t, mockCallbackProxyAddress, req.GetProxyAddressForSso())
		require.Equal(t, mockCallbackURL, req.GetBrowserMfaTshRedirectUrl())

		resp := newSessionBindingCreateResp(mockChallengeName)
		resp.MfaChallenge.SsoChallenge = newMFASSOChallenge()

		return resp, nil
	}
	config.PromptConstructor = func(opts ...mfa.PromptOpt) mfa.Prompt {
		cfg := new(mfa.PromptConfig)
		for _, opt := range opts {
			opt(cfg)
		}

		return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
			require.NotNil(t, cfg.CallbackCeremony)

			return cfg.CallbackCeremony.Run(ctx, chal)
		})
	}

	ceremony, err := mfa.NewSessionBoundCeremony(config)
	require.NoError(t, err)

	gotName, err := ceremony.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, mockChallengeName, gotName)
}

func TestSessionBoundCeremonyRunErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		buildConfig func() mfa.SessionBoundCeremonyConfig
		wantErr     error
	}{
		{
			name: "create session challenge returns error",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					return nil, trace.ConnectionProblem(nil, "create failed")
				}

				return config
			},
			wantErr: trace.ConnectionProblem(nil, "create failed"),
		},
		{
			name: "prompt returns error",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					resp := newSessionBindingCreateResp(mockChallengeName)
					resp.MfaChallenge.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

					return resp, nil
				}
				config.PromptConstructor = func(_ ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(
						func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
							return nil, trace.AccessDenied("prompt failed")
						},
					)
				}

				return config
			},
			wantErr: trace.AccessDenied("prompt failed"),
		},
		{
			name: "missing authenticate challenge in create response",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					return &mfav1.CreateSessionChallengeResponse{}, nil
				}

				return config
			},
			wantErr: trace.BadParameter("AuthenticateChallenge must not be nil"),
		},
		{
			name: "prompt returns nil response payload",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					resp := newSessionBindingCreateResp(mockChallengeName)
					resp.MfaChallenge.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

					return resp, nil
				}
				config.PromptConstructor = func(_ ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(
						func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
							return nil, nil
						},
					)
				}

				return config
			},
			wantErr: trace.BadParameter("MFAAuthenticateResponse must not be nil"),
		},
		{
			name: "prompt returns response with empty challenge name",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					resp := newSessionBindingCreateResp("")
					resp.MfaChallenge.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

					return resp, nil
				}
				config.PromptConstructor = func(_ ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(
						func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
							return &proto.MFAAuthenticateResponse{
								Response: &proto.MFAAuthenticateResponse_Browser{
									Browser: newProtoBrowserResponse("", nil),
								},
							}, nil
						},
					)
				}

				return config
			},
			wantErr: trace.BadParameter("challenge name must not be empty"),
		},
		{
			name: "validate session challenge returns error",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
					resp := newSessionBindingCreateResp(mockChallengeName)
					resp.MfaChallenge.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

					return resp, nil
				}
				config.PromptConstructor = func(_ ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(
						func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
							return &proto.MFAAuthenticateResponse{
								Response: &proto.MFAAuthenticateResponse_SSO{
									SSO: newProtoSSOResponse(mockSSOToken),
								},
							}, nil
						},
					)
				}

				config.ValidateSessionChallenge = func(_ context.Context, _ *mfav1.ValidateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.ValidateSessionChallengeResponse, error) {
					return nil, trace.CompareFailed("validate failed")
				}

				return config
			},
			wantErr: trace.CompareFailed("validate failed"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ceremony, err := mfa.NewSessionBoundCeremony(test.buildConfig())
			require.NoError(t, err)

			name, err := ceremony.Run(t.Context())
			require.ErrorIs(t, err, test.wantErr)
			require.Empty(t, name)
		})
	}
}

func newMFASSOChallenge() *mfav1.SSOChallenge {
	return &mfav1.SSOChallenge{
		RequestId:   mockSSORequestID,
		RedirectUrl: mockRedirectURL,
	}
}

func newProtoSSOChallenge() *proto.SSOChallenge {
	return &proto.SSOChallenge{
		RequestId:   mockSSORequestID,
		RedirectUrl: mockRedirectURL,
	}
}

func newWebauthnChallenge(challenge string) *webauthnpb.CredentialAssertion {
	return &webauthnpb.CredentialAssertion{
		PublicKey: &webauthnpb.PublicKeyCredentialRequestOptions{
			Challenge: []byte(challenge),
		},
	}
}

func newWebauthnResponse(rawID string) *webauthnpb.CredentialAssertionResponse {
	return &webauthnpb.CredentialAssertionResponse{
		RawId: []byte(rawID),
	}
}

func newProtoSSOResponse(token string) *proto.SSOResponse {
	return &proto.SSOResponse{
		RequestId: mockSSORequestID,
		Token:     token,
	}
}

func newProtoBrowserResponse(requestID string, webauthnResp *webauthnpb.CredentialAssertionResponse) *proto.BrowserMFAResponse {
	return &proto.BrowserMFAResponse{
		RequestId:        requestID,
		WebauthnResponse: webauthnResp,
	}
}

func newMFABrowserResponse(requestID string, webauthnResp *webauthnpb.CredentialAssertionResponse) *mfav1.BrowserMFAResponse {
	return &mfav1.BrowserMFAResponse{
		RequestId:        requestID,
		WebauthnResponse: webauthnResp,
	}
}

func newSessionBindingConfig() mfa.SessionBoundCeremonyConfig {
	return newValidSessionBindingConfig()
}

func newSessionBindingCreateResp(name string) *mfav1.CreateSessionChallengeResponse {
	return &mfav1.CreateSessionChallengeResponse{
		MfaChallenge: &mfav1.AuthenticateChallenge{
			Name: name,
		},
	}
}

func newProtoMFAChallenge() *proto.MFAAuthenticateChallenge {
	return &proto.MFAAuthenticateChallenge{}
}

func newValidSessionBindingConfig() mfa.SessionBoundCeremonyConfig {
	return mfa.SessionBoundCeremonyConfig{
		CreateSessionChallenge: func(_ context.Context, _ *mfav1.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
			resp := newSessionBindingCreateResp(mockChallengeName)
			resp.MfaChallenge.WebauthnChallenge = newWebauthnChallenge(mockWebauthnChallenge)

			return resp, nil
		},
		PromptConstructor: func(_ ...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: newWebauthnResponse(mockCredentialID),
					},
				}, nil
			})
		},
		ValidateSessionChallenge: func(_ context.Context, _ *mfav1.ValidateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav1.ValidateSessionChallengeResponse, error) {
			return &mfav1.ValidateSessionChallengeResponse{}, nil
		},
		Payload:       &mfav1.SessionIdentifyingPayload{},
		TargetCluster: mockTargetCluster,
	}
}

type mockMFACeremony struct {
	clientCallbackURL string
	proxyAddress      string
	prompt            mfa.PromptFunc
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

func (m *mockMFACeremony) Close() {}
