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
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// TestHostCredentialsHttpFallback tests that HostCredentials requests (/v1/webapi/host/credentials/)
// fall back to HTTP only if the address is a loopback and the insecure mode was set.
func TestHostCredentialsHttpFallback(t *testing.T) {
	testCases := []struct {
		desc     string
		loopback bool
		insecure bool
		fallback bool
	}{
		{
			desc:     "falls back to http if loopback and insecure",
			loopback: true,
			insecure: true,
			fallback: true,
		},
		{
			desc:     "does not fall back to http if loopback and secure",
			loopback: true,
			insecure: false,
			fallback: false,
		},
		{
			desc:     "does not fall back to http if non-loopback and insecure",
			loopback: false,
			insecure: true,
			fallback: false,
		},
	}

	for _, tc := range testCases {
		// Start an http server (not https) so that the request only succeeds
		// if the fallback occurs.
		var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI != "/webapi/host/credentials" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(proto.Certs{})
		}
		httpSvr, err := newServer(handler, tc.loopback)
		require.NoError(t, err)
		defer httpSvr.Close()

		// Send the HostCredentials request.
		ctx := context.Background()
		_, err = client.HostCredentials(ctx, httpSvr.Listener.Addr().String(), tc.insecure, types.RegisterUsingTokenRequest{})

		// If it should fallback, then no error should occur
		// as the request will hit the running http server.
		if tc.fallback {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

// newServer starts a new server that uses a loopback listener if `loopback`.
func newServer(handler http.HandlerFunc, loopback bool) (*httptest.Server, error) {
	srv := httptest.NewUnstartedServer(handler)

	if !loopback {
		// Replace the test-supplied loopback listener with the first available
		// non-loopback address.
		srv.Listener.Close()
		l, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			return nil, err
		}
		srv.Listener = l
	}

	srv.Start()
	return srv, nil
}

func TestSSHAgentPasswordlessLogin(t *testing.T) {
	t.Parallel()

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

	solvePwdless := func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
		car, err := device.SignAssertion(origin, assertion)
		if err != nil {
			return nil, err
		}
		resp := &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wantypes.CredentialAssertionResponseToProto(car),
			},
		}
		resp.GetWebauthn().Response.UserHandle = webID

		return resp, nil
	}

	tc, err := client.NewClient(cfg)
	require.NoError(t, err)

	userKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(userKey.Public())
	require.NoError(t, err)
	tlsPub, err := keys.MarshalPublicKey(userKey.Public())
	require.NoError(t, err)

	// customPromptCalled is a flag to ensure the custom prompt was indeed called
	// for each test.
	customPromptCalled := false

	tests := []struct {
		name                 string
		customPromptWebauthn func(ctx context.Context, origin string, assert *wantypes.CredentialAssertion, p wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
		customPromptLogin    wancli.LoginPrompt
	}{
		{
			name: "with custom prompt",
			customPromptWebauthn: func(ctx context.Context, origin string, assert *wantypes.CredentialAssertion, p wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
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

				ackTouch, err := p.PromptTouch()
				require.NoError(t, err)

				resp, err := solvePwdless(ctx, origin, assert, p)
				if err != nil {
					return nil, "", err
				}
				return resp, "", ackTouch()
			},
			customPromptLogin: &customPromptLogin{},
		},
		{
			name: "without custom prompt",
			customPromptWebauthn: func(ctx context.Context, origin string, assert *wantypes.CredentialAssertion, p wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
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
				SSHPubKey:         ssh.MarshalAuthorizedKey(sshPub),
				TLSPubKey:         tlsPub,
				TTL:               tc.KeyTTL,
				Insecure:          tc.InsecureSkipVerify,
				Compatibility:     tc.CertificateFormat,
				RouteToCluster:    tc.SiteName,
				KubernetesCluster: tc.KubernetesCluster,
			},
			AuthenticatorAttachment: tc.AuthenticatorAttachment,
			CustomPrompt:            test.customPromptLogin,
			WebauthnLogin:           test.customPromptWebauthn,
		}

		_, err = client.SSHAgentPasswordlessLogin(ctx, req)
		require.NoError(t, err)
		require.True(t, customPromptCalled, "Custom prompt present but not called")
	}
}

type customPromptLogin struct{}

func (p *customPromptLogin) PromptPIN() (string, error) {
	return "", nil
}

func (p *customPromptLogin) PromptTouch() (wancli.TouchAcknowledger, error) {
	return func() error { return nil }, nil
}

func (p *customPromptLogin) PromptCredential(deviceCreds []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
	return nil, nil
}
