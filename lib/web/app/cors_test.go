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
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/readonly"
)

type fakeWatcher struct {
	servers []types.AppServer
	err     error
}

func (f fakeWatcher) CurrentResourcesWithFilter(_ context.Context, _ func(readonly.AppServer) bool) ([]types.AppServer, error) {
	return f.servers, f.err
}

func TestHandlePreflight(t *testing.T) {
	const (
		targetAppURL   = "https://my-app.example.com/"
		allowedOrigin  = "https://allowed.example.com"
		deniedOrigin   = "https://denied.example.com"
		wildcardOrigin = "https://wildcard.example.com"
	)

	makeServer := func(t *testing.T, cors *types.CORSPolicy) types.AppServer {
		app, err := types.NewAppV3(types.Metadata{Name: "my-app"}, types.AppSpecV3{
			URI:  "http://localhost",
			CORS: cors,
		})
		require.NoError(t, err)
		s, err := types.NewAppServerV3FromApp(app, "host", "hostID")
		require.NoError(t, err)
		return s
	}

	fullPolicy := &types.CORSPolicy{
		AllowedOrigins:   []string{allowedOrigin},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"X-Custom"},
		ExposedHeaders:   []string{"X-Expose"},
		AllowCredentials: true,
		MaxAge:           600,
	}

	tests := []struct {
		name        string
		lookup      fakeWatcher
		origin      string
		wantHeaders map[string]string
	}{
		{
			name:   "allowed origin gets full CORS headers",
			lookup: fakeWatcher{servers: []types.AppServer{makeServer(t, fullPolicy)}},
			origin: allowedOrigin,
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin":      allowedOrigin,
				"Access-Control-Allow-Methods":     "GET,POST",
				"Access-Control-Allow-Headers":     "X-Custom",
				"Access-Control-Expose-Headers":    "X-Expose",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           "600",
			},
		},
		{
			name: "wildcard policy echoes request origin",
			lookup: fakeWatcher{servers: []types.AppServer{makeServer(t, &types.CORSPolicy{
				AllowedOrigins: []string{"*"},
			})}},
			origin: wildcardOrigin,
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin": wildcardOrigin,
			},
		},
		{
			name:        "denied origin gets no CORS headers",
			lookup:      fakeWatcher{servers: []types.AppServer{makeServer(t, fullPolicy)}},
			origin:      deniedOrigin,
			wantHeaders: map[string]string{"Access-Control-Allow-Origin": ""},
		},
		{
			name:        "app without CORS policy gets no headers",
			lookup:      fakeWatcher{servers: []types.AppServer{makeServer(t, nil)}},
			origin:      allowedOrigin,
			wantHeaders: map[string]string{"Access-Control-Allow-Origin": ""},
		},
		{
			name:        "no matching servers",
			lookup:      fakeWatcher{},
			origin:      allowedOrigin,
			wantHeaders: map[string]string{"Access-Control-Allow-Origin": ""},
		},
		{
			name:        "lookup error",
			lookup:      fakeWatcher{err: trace.ConnectionProblem(nil, "boom")},
			origin:      allowedOrigin,
			wantHeaders: map[string]string{"Access-Control-Allow-Origin": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				c:      &HandlerConfig{AppServerWatcher: tt.lookup},
				logger: slog.Default(),
			}

			req := httptest.NewRequest(http.MethodOptions, targetAppURL, nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			h.HandlePreflight(rec, req)

			for k, v := range tt.wantHeaders {
				require.Equal(t, v, rec.Header().Get(k), "header %s", k)
			}
		})
	}
}
