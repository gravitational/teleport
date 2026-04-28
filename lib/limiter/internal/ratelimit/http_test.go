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

package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestServeHTTP_RetryAfter(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: 10 * time.Second, average: 1, burst: 1})
	clock := clockwork.NewFakeClock()
	tl, err := New(TokenLimiterConfig{Rates: rates, Clock: clock})
	require.NoError(t, err)
	tl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	doRequest := func() *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:9999"
		w := httptest.NewRecorder()
		tl.ServeHTTP(w, req)
		return w
	}

	first := doRequest()
	require.Equal(t, http.StatusOK, first.Code)

	second := doRequest()
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Equal(t, "11", second.Header().Get("Retry-After"))

	clock.Advance(5 * time.Second)
	third := doRequest()
	require.Equal(t, http.StatusTooManyRequests, third.Code)
	require.Equal(t, "6", third.Header().Get("Retry-After"))

	clock.Advance(5 * time.Second)
	fourth := doRequest()
	require.Equal(t, http.StatusOK, fourth.Code)
}
