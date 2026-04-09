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
	"github.com/gravitational/teleport/api/client/webclient"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
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

func TestMFACeremony_DoesNotOfferRegistrationOutsideSessionMFA(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var askedRegister bool
	ceremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			return &proto.MFAAuthenticateChallenge{}, nil
		},
		CreateRegisterChallenge: func(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
			t.Fatal("unexpected registration challenge")
			return nil, nil
		},
		AddMFADevice: func(ctx context.Context, req *proto.MFARegisterResponse, config mfa.RegistrationCeremonyConfig) error {
			t.Fatal("unexpected AddMFADevice")
			return nil
		},
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			return &mockPrompt{
				run: func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
					return &proto.MFAAuthenticateResponse{}, nil
				},
				askRegister: func(ctx context.Context, config mfa.RegistrationPromptConfig) (*mfa.RegistrationPromptConfig, error) {
					askedRegister = true
					return nil, nil
				},
			}
		},
	}

	resp, err := ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
		},
	})
	require.NoError(t, err)
	require.Equal(t, &proto.MFAAuthenticateResponse{}, resp)
	require.False(t, askedRegister)
}

func TestMFACeremony_DeclinedRegistrationReturnsNoop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ceremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			return &proto.MFAAuthenticateChallenge{}, nil
		},
		Ping: func(ctx context.Context) (*webclient.PingResponse, error) {
			return &webclient.PingResponse{}, nil
		},
		CreateRegisterChallenge: func(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
			return &proto.MFARegisterChallenge{}, nil
		},
		AddMFADevice: func(ctx context.Context, req *proto.MFARegisterResponse, config mfa.RegistrationCeremonyConfig) error {
			t.Fatal("unexpected AddMFADevice")
			return nil
		},
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			return &mockPrompt{
				run: func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
					t.Fatal("unexpected MFA prompt after registration declined")
					return nil, nil
				},
				askRegister: func(ctx context.Context, config mfa.RegistrationPromptConfig) (*mfa.RegistrationPromptConfig, error) {
					return nil, nil
				},
			}
		},
	}

	resp, err := ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
	})
	require.NoError(t, err)
	require.Equal(t, &proto.MFAAuthenticateResponse{}, resp)
}

func TestMFACeremony_Register(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	authResp := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: "123456"},
		},
	}
	registerResp := &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: "654321", ID: "reg-id"},
		},
	}
	registerChal := &proto.MFARegisterChallenge{
		Request: &proto.MFARegisterChallenge_TOTP{
			TOTP: &proto.TOTPRegisterChallenge{Digits: 6},
		},
	}

	var promptDescriptors []mfa.DeviceDescriptor
	callbacks := &mockRegistrationCallbacks{}
	var notified bool

	ceremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			require.NotNil(t, req)
			require.Equal(t, mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES, req.ChallengeExtensions.GetScope())
			return &proto.MFAAuthenticateChallenge{TOTP: &proto.TOTPChallenge{}}, nil
		},
		CreateRegisterChallenge: func(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
			require.Equal(t, authResp, req.ExistingMFAResponse)
			require.Equal(t, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, req.DeviceType)
			require.Equal(t, proto.DeviceUsage_DEVICE_USAGE_MFA, req.DeviceUsage)
			return registerChal, nil
		},
		AddMFADevice: func(ctx context.Context, req *proto.MFARegisterResponse, config mfa.RegistrationCeremonyConfig) error {
			require.Equal(t, registerResp, req)
			require.Equal(t, "new-device", config.DeviceName)
			require.Equal(t, mfa.MFADeviceTypeWebauthn, config.DeviceType)
			require.Equal(t, proto.DeviceUsage_DEVICE_USAGE_MFA, config.DeviceUsage)
			return nil
		},
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			cfg := new(mfa.PromptConfig)
			for _, opt := range opts {
				opt(cfg)
			}
			promptDescriptors = append(promptDescriptors, cfg.DeviceType)

			switch cfg.DeviceType {
			case "":
				return &mockPrompt{
					askRegister: func(ctx context.Context, config mfa.RegistrationPromptConfig) (*mfa.RegistrationPromptConfig, error) {
						return &config, nil
					},
				}
			case mfa.DeviceDescriptorRegistered:
				return &mockPrompt{
					run: func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						require.NotNil(t, chal.GetTOTP())
						return authResp, nil
					},
				}
			case mfa.DeviceDescriptorNew:
				return &mockPrompt{
					runRegister: func(ctx context.Context, config mfa.RegistrationPromptConfig, chal *proto.MFARegisterChallenge) (*mfa.RegistrationResult, error) {
						require.Equal(t, registerChal, chal)
						return &mfa.RegistrationResult{
							Config:    config,
							Response:  registerResp,
							Callbacks: callbacks,
						}, nil
					},
					notifyRegistrationSuccess: func(ctx context.Context, config mfa.RegistrationPromptConfig) error {
						notified = true
						return nil
					},
				}
			default:
				t.Fatalf("unexpected device descriptor %q", cfg.DeviceType)
				return nil
			}
		},
	}

	added, err := ceremony.Register(ctx, mfa.RegistrationCeremonyConfig{
		DeviceName:  "new-device",
		DeviceType:  mfa.MFADeviceTypeWebauthn,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_MFA,
	})
	require.NoError(t, err)
	require.True(t, added)
	require.Equal(t, []mfa.DeviceDescriptor{"", mfa.DeviceDescriptorRegistered, mfa.DeviceDescriptorNew}, promptDescriptors)
	require.True(t, callbacks.confirmed)
	require.False(t, callbacks.rolledBack)
	require.True(t, notified)
}

type mockPrompt struct {
	run                       func(context.Context, *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)
	askRegister               func(context.Context, mfa.RegistrationPromptConfig) (*mfa.RegistrationPromptConfig, error)
	runRegister               func(context.Context, mfa.RegistrationPromptConfig, *proto.MFARegisterChallenge) (*mfa.RegistrationResult, error)
	notifyRegistrationSuccess func(context.Context, mfa.RegistrationPromptConfig) error
}

func (m *mockPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if m.run == nil {
		return nil, trace.NotImplemented("not supported")
	}
	return m.run(ctx, chal)
}

func (m *mockPrompt) AskRegister(ctx context.Context, config mfa.RegistrationPromptConfig) (*mfa.RegistrationPromptConfig, error) {
	if m.askRegister == nil {
		return nil, trace.NotImplemented("not supported")
	}
	return m.askRegister(ctx, config)
}

func (m *mockPrompt) RunRegister(ctx context.Context, config mfa.RegistrationPromptConfig, chal *proto.MFARegisterChallenge) (*mfa.RegistrationResult, error) {
	if m.runRegister == nil {
		return nil, trace.NotImplemented("not supported")
	}
	return m.runRegister(ctx, config, chal)
}

func (m *mockPrompt) NotifyRegistrationSuccess(ctx context.Context, config mfa.RegistrationPromptConfig) error {
	if m.notifyRegistrationSuccess == nil {
		return nil
	}
	return m.notifyRegistrationSuccess(ctx, config)
}

type mockRegistrationCallbacks struct {
	confirmed  bool
	rolledBack bool
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
	prompt            mfa.PromptFunc
}

// GetClientCallbackURL returns the client callback URL.
func (m *mockMFACeremony) GetClientCallbackURL() string {
	return m.clientCallbackURL
}

func (m *mockMFACeremony) GetProxyAddress() string {
	return ""
}

// Run the SSO MFA ceremony.
func (m *mockMFACeremony) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return m.prompt(ctx, chal)
}

func (m *mockMFACeremony) Close() {}
