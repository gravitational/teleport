/*
Copyright 2026 Gravitational, Inc.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	webauthnv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/webauthn/v2"
	"github.com/gravitational/teleport/api/mfa"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
)

const (
	mockTargetCluster        = "root"
	mockRedirectURL          = "https://example.com/redirect"
	mockCallbackURL          = "https://example.com/callback"
	mockCallbackProxyAddress = "proxy.example.com:3080"
	mockProxyAddress         = "proxy.example.com:443"
	mockSSORequestID         = "sso-request-id"
	mockSSOToken             = "sso-token"
	mockChallengeName        = "challenge"
	mockWebauthnChallenge    = "webauthn-challenge"
	mockCredentialID         = "credential-id"
	mockBrowserRequestID     = "browser-request-id"
	mockBrowserCredentialID  = "browser-credential-id"
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
			assert.ErrorIs(t, err, test.wantErr)
			assert.Empty(t, ceremony)
		})
	}
}

func TestSessionBoundCeremonyRun(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                        string
		createResp                  *mfav2.CreateSessionChallengeResponse
		promptResp                  *proto.MFAAuthenticateResponse
		wantPromptChal              *proto.MFAAuthenticateChallenge
		wantValidateResp            *mfav2.AuthenticateResponse
		callbackCeremonyConstructor mfa.MFACeremonyConstructor
	}{
		{
			name: "webauthn response",
			createResp: mfav2.CreateSessionChallengeResponse_builder{
				MfaChallenge: mfav2.AuthenticateChallenge_builder{
					Name:              "challenge-webauthn",
					WebauthnChallenge: newWebauthnChallenge(mockWebauthnChallenge),
				}.Build(),
			}.Build(),
			promptResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: newProtoWebauthnResponse(mockCredentialID),
				},
			},
			wantPromptChal: func() *proto.MFAAuthenticateChallenge {
				chal := newProtoMFAChallenge()
				chal.WebauthnChallenge = newProtoWebauthnChallenge(mockWebauthnChallenge)

				return chal
			}(),
			wantValidateResp: mfav2.AuthenticateResponse_builder{
				Name:     "challenge-webauthn",
				Webauthn: newWebauthnResponse(mockCredentialID),
			}.Build(),
		},
		{
			name: "sso response",
			createResp: mfav2.CreateSessionChallengeResponse_builder{
				MfaChallenge: mfav2.AuthenticateChallenge_builder{
					Name:         "challenge-sso",
					SsoChallenge: newMFASSOChallenge(),
				}.Build(),
			}.Build(),
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
			wantValidateResp: mfav2.AuthenticateResponse_builder{
				Name: "challenge-sso",
				Sso: mfav2.SSOChallengeResponse_builder{
					RequestId: mockSSORequestID,
					Token:     mockSSOToken,
				}.Build(),
			}.Build(),
			callbackCeremonyConstructor: func(context.Context) (mfa.CallbackCeremony, error) {
				return &mockMFACeremony{
					clientCallbackURL: mockCallbackURL,
					proxyAddress:      mockProxyAddress,
				}, nil
			},
		},
		{
			name: "browser response",
			createResp: mfav2.CreateSessionChallengeResponse_builder{
				MfaChallenge: mfav2.AuthenticateChallenge_builder{
					Name: "challenge-browser",
					BrowserChallenge: mfav2.BrowserMFAChallenge_builder{
						RequestId: mockBrowserRequestID,
					}.Build(),
				}.Build(),
			}.Build(),
			promptResp: &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Browser{
					Browser: newProtoBrowserResponse(mockBrowserRequestID, newProtoWebauthnResponse(mockBrowserCredentialID)),
				},
			},
			wantPromptChal: func() *proto.MFAAuthenticateChallenge {
				chal := newProtoMFAChallenge()
				chal.BrowserMFAChallenge = &proto.BrowserMFAChallenge{
					RequestId: mockBrowserRequestID,
				}

				return chal
			}(),
			wantValidateResp: mfav2.AuthenticateResponse_builder{
				Name:    "challenge-browser",
				Browser: newMFABrowserResponse(mockBrowserRequestID, newWebauthnResponse(mockBrowserCredentialID)),
			}.Build(),
			callbackCeremonyConstructor: func(context.Context) (mfa.CallbackCeremony, error) {
				return &mockMFACeremony{
					clientCallbackURL: mockCallbackURL,
					proxyAddress:      mockProxyAddress,
				}, nil
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := newSessionBindingConfig()

			ceremony, err := mfa.NewSessionBoundCeremony(mfa.SessionBoundCeremonyConfig{
				CreateSessionChallenge: func(_ context.Context, req *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
					require.Equal(t, config.TargetCluster, req.GetTargetCluster())

					if test.callbackCeremonyConstructor == nil {
						require.Empty(t, req.GetSsoClientRedirectUrl())
						require.Empty(t, req.GetProxyAddressForSso())
						require.Empty(t, req.GetBrowserMfaTshRedirectUrl())
					} else {
						require.Equal(t, mockCallbackURL, req.GetSsoClientRedirectUrl())
						require.Equal(t, mockProxyAddress, req.GetProxyAddressForSso())
						require.Equal(t, mockCallbackURL, req.GetBrowserMfaTshRedirectUrl())
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

							if test.callbackCeremonyConstructor == nil {
								require.Nil(t, cfg.CallbackCeremony)
							} else {
								require.NotNil(t, cfg.CallbackCeremony)
							}

							return test.promptResp, nil
						},
					)
				},
				ValidateSessionChallenge: func(_ context.Context, req *mfav2.ValidateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.ValidateSessionChallengeResponse, error) {
					require.Empty(t, cmp.Diff(test.wantValidateResp, req.GetMfaResponse(), protocmp.Transform()))

					return &mfav2.ValidateSessionChallengeResponse{}, nil
				},
				CallbackCeremonyConstructor: test.callbackCeremonyConstructor,
				TargetCluster:               config.TargetCluster,
			})
			require.NoError(t, err)

			gotName, err := ceremony.Run(t.Context(), &mfav2.SessionIdentifyingPayload{})
			require.NoError(t, err)
			require.Equal(t, test.createResp.GetMfaChallenge().GetName(), gotName, "expected challenge name to be returned")
		})
	}
}

func TestSessionBoundCeremonyRun_CallbackCeremony(t *testing.T) {
	t.Parallel()

	wantResp := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_SSO{
			SSO: newProtoSSOResponse(mockSSOToken),
		},
	}

	config := newSessionBindingConfig()
	config.CallbackCeremonyConstructor = func(context.Context) (mfa.CallbackCeremony, error) {
		return &mockMFACeremony{
			clientCallbackURL: mockCallbackURL,
			proxyAddress:      mockCallbackProxyAddress,
			prompt: func(_ context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				require.NotNil(t, chal.GetSSOChallenge())

				return wantResp, nil
			},
		}, nil
	}
	config.CreateSessionChallenge = func(_ context.Context, req *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
		require.Equal(t, mockCallbackURL, req.GetSsoClientRedirectUrl())
		require.Equal(t, mockCallbackProxyAddress, req.GetProxyAddressForSso())
		require.Equal(t, mockCallbackURL, req.GetBrowserMfaTshRedirectUrl())

		return mfav2.CreateSessionChallengeResponse_builder{
			MfaChallenge: mfav2.AuthenticateChallenge_builder{
				Name:         mockChallengeName,
				SsoChallenge: newMFASSOChallenge(),
			}.Build(),
		}.Build(), nil
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

	gotName, err := ceremony.Run(t.Context(), &mfav2.SessionIdentifyingPayload{})
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
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
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
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
					return mfav2.CreateSessionChallengeResponse_builder{
						MfaChallenge: mfav2.AuthenticateChallenge_builder{
							Name:              mockChallengeName,
							WebauthnChallenge: newWebauthnChallenge(mockWebauthnChallenge),
						}.Build(),
					}.Build(), nil
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
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
					return &mfav2.CreateSessionChallengeResponse{}, nil
				}

				return config
			},
			wantErr: trace.BadParameter("AuthenticateChallenge must not be nil"),
		},
		{
			name: "prompt returns response with empty challenge name",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
					return mfav2.CreateSessionChallengeResponse_builder{
						MfaChallenge: mfav2.AuthenticateChallenge_builder{
							Name:              "",
							WebauthnChallenge: newWebauthnChallenge(mockWebauthnChallenge),
						}.Build(),
					}.Build(), nil
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
			name: "prompt returns unsupported or nil MFA response",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
					return mfav2.CreateSessionChallengeResponse_builder{
						MfaChallenge: mfav2.AuthenticateChallenge_builder{
							Name:              mockChallengeName,
							WebauthnChallenge: newWebauthnChallenge(mockWebauthnChallenge),
						}.Build(),
					}.Build(), nil
				}
				config.PromptConstructor = func(_ ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(
						func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
							return &proto.MFAAuthenticateResponse{}, nil
						},
					)
				}

				return config
			},
			wantErr: trace.BadParameter("unsupported MFA response from client (type <nil>); update your client to the latest supported version for this cluster and try again"),
		},
		{
			name: "validate session challenge returns error",
			buildConfig: func() mfa.SessionBoundCeremonyConfig {
				config := newSessionBindingConfig()
				config.CreateSessionChallenge = func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
					return mfav2.CreateSessionChallengeResponse_builder{
						MfaChallenge: mfav2.AuthenticateChallenge_builder{
							Name:              mockChallengeName,
							WebauthnChallenge: newWebauthnChallenge(mockWebauthnChallenge),
						}.Build(),
					}.Build(), nil
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

				config.ValidateSessionChallenge = func(_ context.Context, _ *mfav2.ValidateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.ValidateSessionChallengeResponse, error) {
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

			name, err := ceremony.Run(t.Context(), &mfav2.SessionIdentifyingPayload{})
			require.ErrorIs(t, err, test.wantErr)
			require.Empty(t, name)
		})
	}
}

func newMFASSOChallenge() *mfav2.SSOChallenge {
	return mfav2.SSOChallenge_builder{
		RequestId:   mockSSORequestID,
		RedirectUrl: mockRedirectURL,
	}.Build()
}

func newProtoSSOChallenge() *proto.SSOChallenge {
	return &proto.SSOChallenge{
		RequestId:   mockSSORequestID,
		RedirectUrl: mockRedirectURL,
	}
}

func newWebauthnChallenge(challenge string) *webauthnv2.CredentialAssertion {
	return webauthnv2.CredentialAssertion_builder{
		PublicKey: webauthnv2.PublicKeyCredentialRequestOptions_builder{
			Challenge: []byte(challenge),
		}.Build(),
	}.Build()
}

func newProtoWebauthnChallenge(challenge string) *webauthnpb.CredentialAssertion {
	return &webauthnpb.CredentialAssertion{
		PublicKey: &webauthnpb.PublicKeyCredentialRequestOptions{
			Challenge: []byte(challenge),
		},
	}
}

func newWebauthnResponse(rawID string) *webauthnv2.CredentialAssertionResponse {
	return webauthnv2.CredentialAssertionResponse_builder{
		RawId:    []byte(rawID),
		Response: webauthnv2.AuthenticatorAssertionResponse_builder{}.Build(),
	}.Build()
}

func newProtoWebauthnResponse(rawID string) *webauthnpb.CredentialAssertionResponse {
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

func newMFABrowserResponse(requestID string, webauthnResp *webauthnv2.CredentialAssertionResponse) *mfav2.BrowserMFAResponse {
	return mfav2.BrowserMFAResponse_builder{
		RequestId:        requestID,
		WebauthnResponse: webauthnResp,
	}.Build()
}

func newProtoMFAChallenge() *proto.MFAAuthenticateChallenge {
	return &proto.MFAAuthenticateChallenge{}
}

func newSessionBindingConfig() mfa.SessionBoundCeremonyConfig {
	return mfa.SessionBoundCeremonyConfig{
		CreateSessionChallenge: func(_ context.Context, _ *mfav2.CreateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error) {
			return mfav2.CreateSessionChallengeResponse_builder{
				MfaChallenge: mfav2.AuthenticateChallenge_builder{
					Name:              mockChallengeName,
					WebauthnChallenge: newWebauthnChallenge(mockWebauthnChallenge),
				}.Build(),
			}.Build(), nil
		},
		PromptConstructor: func(_ ...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(_ context.Context, _ *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: newProtoWebauthnResponse(mockCredentialID),
					},
				}, nil
			})
		},
		ValidateSessionChallenge: func(_ context.Context, _ *mfav2.ValidateSessionChallengeRequest, _ ...grpc.CallOption) (*mfav2.ValidateSessionChallengeResponse, error) {
			return &mfav2.ValidateSessionChallengeResponse{}, nil
		},
		TargetCluster: mockTargetCluster,
	}
}
