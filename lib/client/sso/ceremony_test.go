/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package sso_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/web"
)

func TestCLICeremony(t *testing.T) {
	ctx := context.Background()

	mockProxy := newMockProxy(t)
	username := "alice"

	// Capture stderr.
	stderr := &bytes.Buffer{}

	// Create a basic redirector.
	rd, err := sso.NewRedirector(sso.RedirectorConfig{
		ProxyAddr: mockProxy.URL,
		Browser:   teleport.BrowserNone,
		Stderr:    stderr,
	})
	require.NoError(t, err)
	t.Cleanup(rd.Close)

	// Construct a fake ssh login response with the redirector's client callback URL.
	successResponseURL, err := web.ConstructSSHResponse(web.AuthParams{
		ClientRedirectURL: rd.ClientCallbackURL,
		Username:          username,
	})
	require.NoError(t, err)

	// Open a mock IdP server which will handle a redirect and result in the expected IdP session payload.
	mockIdPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, successResponseURL.String(), http.StatusPermanentRedirect)
	}))
	t.Cleanup(mockIdPServer.Close)

	ceremony := sso.NewCLICeremony(rd, func(ctx context.Context, clientCallbackURL string) (redirectURL string, err error) {
		return mockIdPServer.URL, nil
	})

	// Modify handle redirect to also browse to the clickable URL printed to stderr.
	baseHandleRedirect := ceremony.HandleRedirect
	ceremony.HandleRedirect = func(ctx context.Context, redirectURL string) error {
		if err := baseHandleRedirect(ctx, redirectURL); err != nil {
			return trace.Wrap(err)
		}

		// Read the clickable url from stderr and navigate to it
		// using a simplified regexp for http://127.0.0.1:<port>/<uuid>
		const clickableURLPattern = `http://127.0.0.1:\d+/[0-9A-Fa-f-]+`
		clickableURL := regexp.MustCompile(clickableURLPattern).FindString(stderr.String())
		resp, err := http.Get(clickableURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		// User should be redirected to success screen.
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, sso.LoginSuccessRedirectURL, string(body))
		return nil
	}

	loginResp, err := ceremony.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, username, loginResp.Username)
}

func TestCLISAMLCeremony(t *testing.T) {
	ctx := context.Background()
	const username = "alice"

	mockProxy := newMockProxy(t)

	for _, tt := range []struct {
		name        string
		redirectURL string
		postForm    string
		assertErr   require.ErrorAssertionFunc
		errExpected bool
	}{
		{
			name:        "handles redirectURL param",
			redirectURL: mockProxy.URL,
			assertErr:   require.NoError,
		},
		{
			name:      "handles postForm param",
			postForm:  base64.StdEncoding.EncodeToString([]byte(postform)),
			assertErr: require.NoError,
		},
		{
			name:        "rejects if both redirectURL and postForm is empty",
			redirectURL: "",
			postForm:    "",
			assertErr:   require.Error,
			errExpected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr.
			stderr := &bytes.Buffer{}

			// Create a basic redirector.
			rd, err := sso.NewRedirector(sso.RedirectorConfig{
				ProxyAddr: mockProxy.URL,
				Stderr:    stderr,
				Browser:   "none",
			})
			require.NoError(t, err)
			t.Cleanup(rd.Close)

			// Construct a fake ssh login response with the redirector's client callback URL.
			successResponseURL, err := web.ConstructSSHResponse(web.AuthParams{
				ClientRedirectURL: rd.ClientCallbackURL,
				Username:          username,
			})
			require.NoError(t, err)

			// Open a mock IdP server which will handle a redirect and result in the expected IdP session payload.
			mockIdPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, successResponseURL.String(), http.StatusPermanentRedirect)
			}))
			t.Cleanup(mockIdPServer.Close)

			ceremony := sso.NewCLISAMLCeremony(rd, func(ctx context.Context, clientCallbackURL string) (redirectURL string, postForm string, err error) {
				return mockIdPServer.URL, base64.StdEncoding.EncodeToString([]byte(postform)), nil
			})

			// Modify handle request to also browse to the clickable URL printed to stderr.
			baseRequestHandler := ceremony.HandleRequest
			ceremony.HandleRequest = func(ctx context.Context, redirectURL, postForm string) error {
				// returned error will be checked on ceremony.Run(ctx) error.
				if err := baseRequestHandler(ctx, tt.redirectURL, tt.postForm); err != nil {
					return trace.Wrap(err)
				}

				redirected := false
				actualRedirectTo := ""
				httpclient := mockIdPServer.Client()
				httpclient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
					redirected = true
					actualRedirectTo = req.URL.String()
					// ignore redirect
					return http.ErrUseLastResponse
				}

				// Read the clickable url from stderr and navigate to it
				// using a simplified regexp for http://127.0.0.1:<port>/<uuid>
				const clickableURLPattern = `http://127.0.0.1:\d+/[0-9A-Fa-f-]+`
				clickableURL := regexp.MustCompile(clickableURLPattern).FindString(stderr.String())
				resp, err := httpclient.Get(clickableURL)
				require.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				// We are only interested to check if a correct response was made based on redirectURL or postForm parameter.
				// For redirectURL parameter, an HTTP 302 redirection with SAML authentication data is expected.
				// For postForm parameter, an HTTP response with HTML that contains SAML authentication data is expected.
				if tt.redirectURL != "" {
					require.True(t, redirected, "redirection failed for response that contains redirectURL")
					require.Equal(t, http.StatusFound, resp.StatusCode)
					require.Equal(t, tt.redirectURL, actualRedirectTo)
				} else {
					require.Equal(t, http.StatusOK, resp.StatusCode)
					// Validate HTML title for the post form response.
					require.Contains(t, string(body), "Teleport SAML Service Provider")
				}

				// Redirect to success screen to continue the test
				resp, err = http.Get(mockIdPServer.URL)
				require.NoError(t, err)
				defer resp.Body.Close()

				// User should be redirected to success screen.
				body, err = io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, sso.LoginSuccessRedirectURL, string(body))

				return nil
			}

			loginResp, err := ceremony.Run(ctx)
			tt.assertErr(t, err)
			if !tt.errExpected {
				require.Equal(t, username, loginResp.Username)
			}
		})
	}
}

const postform = `
 <form method="POST" action="https://example.com" id="SAMLRequestForm" />
`

func TestCLICeremony_MFA(t *testing.T) {
	const token = "sso-mfa-token"
	const requestID = "soo-mfa-request-id"

	ctx := context.Background()
	mockProxy := newMockProxy(t)

	// Capture stderr.
	stderr := bytes.NewBuffer([]byte{})

	// Create a basic redirector.
	rd, err := sso.NewRedirector(sso.RedirectorConfig{
		ProxyAddr: mockProxy.URL,
		Browser:   teleport.BrowserNone,
		Stderr:    stderr,
	})
	require.NoError(t, err)

	// Construct a fake mfa response with the redirector's client callback URL.
	successResponseURL, err := web.ConstructSSHResponse(web.AuthParams{
		ClientRedirectURL: rd.ClientCallbackURL,
		MFAToken:          token,
	})
	require.NoError(t, err)

	// Open a mock IdP server which will handle a redirect and result in the expected IdP session payload.
	mockIdPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, successResponseURL.String(), http.StatusPermanentRedirect)
	}))
	t.Cleanup(mockIdPServer.Close)

	ceremony := sso.NewCLIMFACeremony(rd)
	t.Cleanup(ceremony.Close)

	// Modify handle redirect to also browse to the clickable URL printed to stderr.
	baseHandleRedirect := ceremony.HandleRedirect
	ceremony.HandleRedirect = func(ctx context.Context, redirectURL string) error {
		if err := baseHandleRedirect(ctx, redirectURL); err != nil {
			return trace.Wrap(err)
		}

		// Read the clickable url from stderr and navigate to it
		// using a simplified regexp for http://127.0.0.1:<port>/<uuid>
		clickableURLPattern := "http://127.0.0.1:.*/.*[0-9a-f]"
		clickableURL := regexp.MustCompile(clickableURLPattern).Find(stderr.Bytes())

		resp, err := http.Get(string(clickableURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		// User should be redirected to success screen.
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, sso.LoginSuccessRedirectURL, string(body))
		return nil
	}

	mfaResponse, err := ceremony.Run(ctx, &proto.MFAAuthenticateChallenge{
		SSOChallenge: &proto.SSOChallenge{
			RedirectUrl: mockIdPServer.URL,
			RequestId:   requestID,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, mfaResponse.GetSSO())
	assert.Equal(t, token, mfaResponse.GetSSO().Token)
	assert.Equal(t, requestID, mfaResponse.GetSSO().RequestId)
}
