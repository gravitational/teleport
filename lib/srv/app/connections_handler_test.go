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

package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/limiter"
)

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()

	c := &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			ServiceComponent: teleport.ComponentApp,
		},
	}

	srv, err := c.newHTTPServer("test-cluster")
	require.NoError(t, err)

	// The HTTP server no longer wraps a limiter (limiting is applied at
	// the connection level in handleConnection). Verify that requests
	// are never rejected with 429 by the HTTP handler itself.
	for i := range 5 {
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
			"request %d should not be rate-limited", i+1)
	}
}

func TestConnectionLimiter_MaxConnections(t *testing.T) {
	t.Parallel()

	lim, err := limiter.NewLimiter(limiter.Config{
		MaxConnections: 2,
	})
	require.NoError(t, err)

	// Acquire two connections from the same IP.
	release1, err := lim.RegisterRequestAndConnection("10.0.0.1")
	require.NoError(t, err)
	release2, err := lim.RegisterRequestAndConnection("10.0.0.1")
	require.NoError(t, err)

	// Third connection from same IP is rejected.
	_, err = lim.RegisterRequestAndConnection("10.0.0.1")
	require.Error(t, err)

	// Different IP still works.
	release3, err := lim.RegisterRequestAndConnection("10.0.0.2")
	require.NoError(t, err)

	// Releasing one connection unblocks the original IP.
	release1()
	release4, err := lim.RegisterRequestAndConnection("10.0.0.1")
	require.NoError(t, err)

	release2()
	release3()
	release4()
}

func TestConnectionLimiter_RateLimiting(t *testing.T) {
	t.Parallel()

	lim, err := limiter.NewLimiter(limiter.Config{
		Rates: []limiter.Rate{
			{
				Period:  time.Minute,
				Average: 1,
				Burst:   1,
			},
		},
	})
	require.NoError(t, err)

	// First connection should succeed.
	release, err := lim.RegisterRequestAndConnection("10.0.0.1")
	require.NoError(t, err)
	release()

	// Second connection from same IP within the rate window is rejected.
	_, err = lim.RegisterRequestAndConnection("10.0.0.1")
	require.Error(t, err)
}
