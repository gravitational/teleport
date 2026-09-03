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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/mcputils"
)

type staticRoundTripper struct {
	resp                  *http.Response
	err                   error
	idleConnectionsClosed bool
}

func (t *staticRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return t.resp, t.err
}

func (t *staticRoundTripper) CloseIdleConnections() {
	t.idleConnectionsClosed = true
}

func makeStaticResponse(statusCode int, origin, body string) *http.Response {
	header := make(http.Header)
	if origin != "" {
		header.Set(mcputils.TeleportErrorOriginHeader, origin)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestHTTPServerErrorRoundTripper(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "http://localhost/", nil)

	t.Run("4xx passes through", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusUnauthorized, "", "unauthorized")
		defer resp.Body.Close()
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		got, err := rt.RoundTrip(req)
		require.NoError(t, err)
		defer got.Body.Close()
		require.Same(t, resp, got)
	})

	t.Run("headerless 5xx passes through", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusBadGateway, "", "proxy error")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		got, err := rt.RoundTrip(req)
		require.NoError(t, err)
		defer got.Body.Close()
		require.Same(t, resp, got)
	})

	t.Run("base error passes through", func(t *testing.T) {
		baseErr := errors.New("dial failed")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{err: baseErr}}
		resp, err := rt.RoundTrip(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.ErrorIs(t, err, baseErr)
	})

	t.Run("5xx becomes typed error", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusInternalServerError, mcputils.ErrorOriginAppService, "failed to \n  rewrite headers")
		defer resp.Body.Close()
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		got, err := rt.RoundTrip(req)
		if got != nil {
			defer got.Body.Close()
		}
		require.Error(t, err)
		var httpErr *HTTPServerError
		require.ErrorAs(t, err, &httpErr)
		require.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
		require.Equal(t, mcputils.ErrorOriginAppService, httpErr.Origin)
		require.Equal(t, "failed to rewrite headers", httpErr.Body)
		require.Equal(t, "request failed with status 500 (reported by app-service): failed to rewrite headers", err.Error())
	})
}

func TestRoundTrippersCloseIdleConnections(t *testing.T) {
	t.Parallel()

	base := &staticRoundTripper{}
	transport := &httpServerErrorRoundTripper{base: &authHeaderRoundTripper{
		base: &oauthRetryRoundTripper{base: base},
	}}
	transport.CloseIdleConnections()
	require.True(t, base.idleConnectionsClosed)
}
