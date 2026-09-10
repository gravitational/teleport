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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

type oauthRetryRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f oauthRetryRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
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

	t.Run("buffers a non-replayable body and replays it", func(t *testing.T) {
		var bodies []string
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				bodies = append(bodies, string(body))
				if len(bodies) == 1 {
					return invalidTokenResponse(req), nil
				}
				return response(req, http.StatusOK), nil
			}),
			refreshAuthHeader: func(context.Context, string) (string, error) {
				return "Bearer fresh-token", nil
			},
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost/mcp", io.NopCloser(strings.NewReader("streaming body")))
		require.NoError(t, err)
		require.Nil(t, req.GetBody)

		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, []string{"streaming body", "streaming body"}, bodies)
	})

	t.Run("rejects an oversized non-replayable body", func(t *testing.T) {
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("oversized request must not be forwarded")
				return nil, nil
			}),
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost/mcp", io.NopCloser(strings.NewReader(strings.Repeat("x", teleport.MaxHTTPRequestSize))))
		require.NoError(t, err)

		resp, err := transport.RoundTrip(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.Nil(t, resp)
		require.ErrorIs(t, err, utils.ErrLimitReached)
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

	t.Run("closes response when replay fails", func(t *testing.T) {
		replayErr := errors.New("replay failed")
		body := &closeTrackingBody{Reader: strings.NewReader("Unauthorized")}
		transport := &oauthRetryRoundTripper{
			base: oauthRetryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				resp := invalidTokenResponse(req)
				resp.Body = body
				return resp, nil
			}),
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader("request"))
		require.NoError(t, err)
		req.GetBody = func() (io.ReadCloser, error) { return nil, replayErr }

		resp, err := transport.RoundTrip(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.Nil(t, resp)
		require.ErrorIs(t, err, replayErr)
		require.True(t, body.closed)
	})
}

func TestIsOAuthInvalidTokenResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int
		challenge string
		want      bool
	}{
		{"invalid token", http.StatusUnauthorized, `Bearer error="invalid_token"`, true},
		{"invalid request", http.StatusBadRequest, `Bearer error="invalid_request"`, false},
		{"insufficient scope", http.StatusForbidden, `Bearer error="insufficient_scope"`, false},
		{"missing challenge", http.StatusUnauthorized, "", false},
		{"wrong status", http.StatusInternalServerError, `Bearer error="invalid_token"`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := response(nil, test.status)
			resp.Header.Set("WWW-Authenticate", test.challenge)
			require.Equal(t, test.want, isOAuthInvalidTokenResponse(resp))
			require.NoError(t, resp.Body.Close())
		})
	}
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
