/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type oauthRetryRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f oauthRetryRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOAuthRetryRoundTripper(t *testing.T) {
	t.Run("refreshes and retries replayable request once", func(t *testing.T) {
		var headers, bodies []string
		base := oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			headers = append(headers, req.Header.Get("Authorization"))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			bodies = append(bodies, string(body))
			if len(headers) == 1 {
				return invalidTokenResponse(req), nil
			}
			return response(req, http.StatusOK), nil
		})
		var rejectedHeader string
		retryTransport := &oauthRetryRoundTripper{
			base: base,
			refreshAuthHeader: func(_ context.Context, rejected string) (string, error) {
				rejectedHeader = rejected
				return "Bearer fresh-token", nil
			},
		}
		transport := &authHeaderRoundTripper{
			base: retryTransport,
			getHeader: func(context.Context) (string, error) {
				return "Bearer rejected-token", nil
			},
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"method":"tools/list"}`))
		require.NoError(t, err)

		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "Bearer rejected-token", rejectedHeader)
		require.Equal(t, []string{"Bearer rejected-token", "Bearer fresh-token"}, headers)
		require.Equal(t, []string{`{"method":"tools/list"}`, `{"method":"tools/list"}`}, bodies)
	})

	t.Run("retries at most once", func(t *testing.T) {
		var requests, refreshes int
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return invalidTokenResponse(req), nil
			}),
			refreshAuthHeader: func(context.Context, string) (string, error) {
				refreshes++
				return "Bearer still-rejected", nil
			},
		}
		req, err := http.NewRequest(http.MethodGet, "http://localhost/mcp", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer rejected-token")

		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Equal(t, 2, requests)
		require.Equal(t, 1, refreshes)
	})

	t.Run("does not retry ambiguous unauthorized response", func(t *testing.T) {
		var requests int
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return response(req, http.StatusUnauthorized), nil
			}),
			refreshAuthHeader: func(context.Context, string) (string, error) {
				t.Fatal("refresh must not be called without an invalid_token challenge")
				return "", nil
			},
		}
		req, err := http.NewRequest(http.MethodGet, "http://localhost/mcp", nil)
		require.NoError(t, err)

		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Equal(t, 1, requests)
	})

	t.Run("does not mistake error description for invalid token error", func(t *testing.T) {
		var requests int
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				resp := response(req, http.StatusUnauthorized)
				resp.Header.Set("WWW-Authenticate", `Bearer error_description="the invalid_token marker is informational"`)
				return resp, nil
			}),
			refreshAuthHeader: func(context.Context, string) (string, error) {
				t.Fatal("refresh must not be called for error_description")
				return "", nil
			},
		}
		req, err := http.NewRequest(http.MethodGet, "http://localhost/mcp", nil)
		require.NoError(t, err)

		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Equal(t, 1, requests)
	})

	t.Run("does not retry a non-replayable request body", func(t *testing.T) {
		var requests int
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return invalidTokenResponse(req), nil
			}),
			refreshAuthHeader: func(context.Context, string) (string, error) {
				t.Fatal("refresh must not be called when the request cannot be retried")
				return "", nil
			},
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost/mcp", io.NopCloser(strings.NewReader("streaming body")))
		require.NoError(t, err)
		require.Nil(t, req.GetBody)

		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Equal(t, 1, requests)
	})

	t.Run("returns refresh failure", func(t *testing.T) {
		refreshErr := errors.New("refresh token rejected")
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return invalidTokenResponse(req), nil
			}),
			refreshAuthHeader: func(context.Context, string) (string, error) {
				return "", refreshErr
			},
		}
		req, err := http.NewRequest(http.MethodGet, "http://localhost/mcp", nil)
		require.NoError(t, err)

		resp, err := transport.RoundTrip(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.Nil(t, resp)
		require.ErrorIs(t, err, refreshErr)
	})
}

func invalidTokenResponse(req *http.Request) *http.Response {
	resp := response(req, http.StatusUnauthorized)
	resp.Header.Set("WWW-Authenticate", `Bearer realm="mcp", error="invalid_token"`)
	return resp
}

func response(req *http.Request, status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(http.StatusText(status))),
		Request:    req,
	}
}
