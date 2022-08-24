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

package client_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/client"
	"github.com/jonboulle/clockwork"

	"github.com/stretchr/testify/require"
)

func TestPlainHttpFallback(t *testing.T) {
	testCases := []struct {
		desc            string
		path            string
		handler         http.HandlerFunc
		actionUnderTest func(ctx context.Context, addr string, insecure bool) error
	}{
		{
			desc: "HostCredentials",
			path: "/v1/webapi/host/credentials",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != "/v1/webapi/host/credentials" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(proto.Certs{})
			},
			actionUnderTest: func(ctx context.Context, addr string, insecure bool) error {
				_, err := client.HostCredentials(ctx, addr, insecure, types.RegisterUsingTokenRequest{})
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			ctx := context.Background()

			t.Run("Allowed on insecure & loopback", func(t *testing.T) {
				httpSvr := httptest.NewServer(testCase.handler)
				defer httpSvr.Close()

				err := testCase.actionUnderTest(ctx, httpSvr.Listener.Addr().String(), true /* insecure */)
				require.NoError(t, err)
			})

			t.Run("Denied on secure", func(t *testing.T) {
				httpSvr := httptest.NewServer(testCase.handler)
				defer httpSvr.Close()

				err := testCase.actionUnderTest(ctx, httpSvr.Listener.Addr().String(), false /* secure */)
				require.Error(t, err)
			})

			t.Run("Denied on non-loopback", func(t *testing.T) {
				nonLoopbackSvr := httptest.NewUnstartedServer(testCase.handler)

				// replace the test-supplied loopback listener with the first available
				// non-loopback address
				nonLoopbackSvr.Listener.Close()
				l, err := net.Listen("tcp", "0.0.0.0:0")
				require.NoError(t, err)
				nonLoopbackSvr.Listener = l
				nonLoopbackSvr.Start()
				defer nonLoopbackSvr.Close()

				err = testCase.actionUnderTest(ctx, nonLoopbackSvr.Listener.Addr().String(), true /* insecure */)
				require.Error(t, err)
			})
		})
	}
}

func TestSSHAgentPasswordlessLogin(t *testing.T) {
	silenceLogger(t)

	clock := clockwork.NewFakeClockAt(time.Now())
	sa := newStandaloneTeleport(t, clock)
	webID := sa.WebAuthnID
	device := sa.Device

	ctx := context.Background()

	// Prepare client config, it won't change throughout the test.
	cfg := client.MakeDefaultConfig()
	cfg.AddKeysToAgent = client.AddKeysToAgentNo
	// Replace "127.0.0.1" with "localhost". The proxy address becomes the origin
	// for Webauthn requests, and Webauthn doesn't take IP addresses.
	cfg.WebProxyAddr = strings.Replace(sa.ProxyWebAddr, "127.0.0.1", "localhost", 1 /* n */)
	cfg.KeysDir = t.TempDir()
	cfg.InsecureSkipVerify = true

	// Reset functions after tests.
	oldWebauthn := *client.PromptWebauthn
	t.Cleanup(func() {
		*client.PromptWebauthn = oldWebauthn
	})

	solvePwdless := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
		car, err := device.SignAssertion(origin, assertion)
		if err != nil {
			return nil, err
		}
		resp := &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(car),
			},
		}
		resp.GetWebauthn().Response.UserHandle = webID

		return resp, nil
	}

	tc, err := client.NewClient(cfg)
	require.NoError(t, err)
	key, err := client.GenerateRSAKey()
	require.NoError(t, err)

	// customPromptCalled is a flag to ensure the custom prompt was indeed called
	// for each test.
	customPromptCalled := false

	tests := []struct {
		name                 string
		customPromptWebauthn func(ctx context.Context, origin string, assert *wanlib.CredentialAssertion, p wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
		customPromptLogin    wancli.LoginPrompt
	}{
		{
			name: "with custom prompt",
			customPromptWebauthn: func(ctx context.Context, origin string, assert *wanlib.CredentialAssertion, p wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
				_, ok := p.(*customPromptLogin)
				require.True(t, ok)
				customPromptCalled = true

				// Test custom prompts can be called.
				pin, err := p.PromptPIN()
				require.NoError(t, err)
				require.Empty(t, pin)

				creds, err := p.PromptCredential(nil)
				require.NoError(t, err)
				require.Empty(t, creds)

				require.NoError(t, p.PromptTouch())

				resp, err := solvePwdless(ctx, origin, assert, p)
				return resp, "", err
			},
			customPromptLogin: &customPromptLogin{},
		},
		{
			name: "without custom prompt",
			customPromptWebauthn: func(ctx context.Context, origin string, assert *wanlib.CredentialAssertion, p wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
				_, ok := p.(*wancli.DefaultPrompt)
				require.True(t, ok)
				customPromptCalled = true

				resp, err := solvePwdless(ctx, origin, assert, p)
				return resp, "", err
			},
		},
	}

	for _, test := range tests {
		customPromptCalled = false // reset flag on each test.
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		req := client.SSHLoginPasswordless{
			SSHLogin: client.SSHLogin{
				ProxyAddr:         tc.WebProxyAddr,
				PubKey:            key.MarshalSSHPublicKey(),
				TTL:               tc.KeyTTL,
				Insecure:          tc.InsecureSkipVerify,
				Compatibility:     tc.CertificateFormat,
				RouteToCluster:    tc.SiteName,
				KubernetesCluster: tc.KubernetesCluster,
			},
			AuthenticatorAttachment: tc.AuthenticatorAttachment,
			CustomPrompt:            test.customPromptLogin,
		}

		*client.PromptWebauthn = test.customPromptWebauthn
		_, err = client.SSHAgentPasswordlessLogin(ctx, req)
		require.NoError(t, err)
		require.True(t, customPromptCalled, "Custom prompt present but not called")
	}
}

type customPromptLogin struct{}

func (p *customPromptLogin) PromptPIN() (string, error) {
	return "", nil
}

func (p *customPromptLogin) PromptTouch() error {
	return nil
}

func (p *customPromptLogin) PromptCredential(deviceCreds []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
	return nil, nil
}
