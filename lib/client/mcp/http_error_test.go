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
	resp *http.Response
	err  error
}

func (t *staticRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return t.resp, t.err
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

	t.Run("success passes through", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusOK, "", "ok")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		got, err := rt.RoundTrip(req)
		require.NoError(t, err)
		require.Same(t, resp, got)
	})

	t.Run("4xx passes through", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusUnauthorized, "", "unauthorized")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		got, err := rt.RoundTrip(req)
		require.NoError(t, err)
		require.Same(t, resp, got)
	})

	t.Run("base error passes through", func(t *testing.T) {
		baseErr := errors.New("dial failed")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{err: baseErr}}
		_, err := rt.RoundTrip(req)
		require.ErrorIs(t, err, baseErr)
	})

	t.Run("5xx becomes typed error", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusInternalServerError, mcputils.ErrorOriginAppService, "failed to \n  rewrite headers")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		_, err := rt.RoundTrip(req)
		require.Error(t, err)
		httpErr, ok := AsHTTPServerError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
		require.Equal(t, mcputils.ErrorOriginAppService, httpErr.Origin)
		require.Equal(t, "failed to rewrite headers", httpErr.Body)
		// String fallbacks still match on "status 500".
		require.Contains(t, err.Error(), "status 500")
	})

	t.Run("5xx without origin header", func(t *testing.T) {
		resp := makeStaticResponse(http.StatusBadGateway, "", "Bad Gateway")
		rt := &httpServerErrorRoundTripper{base: &staticRoundTripper{resp: resp}}
		_, err := rt.RoundTrip(req)
		require.Error(t, err)
		httpErr, ok := AsHTTPServerError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusBadGateway, httpErr.StatusCode)
		require.Empty(t, httpErr.Origin)
	})
}
