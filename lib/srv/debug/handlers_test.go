// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package debug

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type mockLeveler struct {
	currentLevel slog.Level
}

func (m *mockLeveler) GetLogLevel() slog.Level {
	return m.currentLevel
}

func (m *mockLeveler) SetLogLevel(level slog.Level) {
	m.currentLevel = level
}

func TestLogLevel(t *testing.T) {
	leveler, ts := makeServer(t)
	statusCode, logLevel := makeRequest(t, ts, http.MethodGet, "/log-level", "")
	require.Equal(t, http.StatusOK, statusCode)
	require.Equal(t, leveler.GetLogLevel().String(), logLevel)

	// Invalid log level
	statusCode, _ = makeRequest(t, ts, http.MethodPut, "/log-level", "RANDOM")
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)

	for _, logLevel := range logutils.SupportedLevelsText {
		t.Run("Set"+logLevel, func(t *testing.T) {
			statusCode, _ := makeRequest(t, ts, http.MethodPut, "/log-level", logLevel)
			require.Equal(t, http.StatusOK, statusCode)

			statusCode, retrievedLogLevel := makeRequest(t, ts, http.MethodGet, "/log-level", "")
			require.Equal(t, http.StatusOK, statusCode)
			require.Equal(t, logLevel, retrievedLogLevel)
		})

		t.Run("SetLower"+logLevel, func(t *testing.T) {
			statusCode, _ := makeRequest(t, ts, http.MethodPut, "/log-level", strings.ToLower(logLevel))
			require.Equal(t, http.StatusOK, statusCode)

			statusCode, retrievedLogLevel := makeRequest(t, ts, http.MethodGet, "/log-level", "")
			require.Equal(t, http.StatusOK, statusCode)
			require.Equal(t, logLevel, retrievedLogLevel)
		})
	}

	// Random or any other slog format should be rejected.
	for _, level := range []string{"RANDOM", "DEBUG-1", "INFO+1", "INVALID"} {
		t.Run("Set"+level, func(t *testing.T) {
			statusCode, _ := makeRequest(t, ts, http.MethodPut, "/log-level", level)
			require.Equal(t, http.StatusUnprocessableEntity, statusCode)
		})

		t.Run("SetLower"+level, func(t *testing.T) {
			statusCode, _ := makeRequest(t, ts, http.MethodPut, "/log-level", strings.ToLower(level))
			require.Equal(t, http.StatusUnprocessableEntity, statusCode)
		})
	}
}

func TestCollectProfiles(t *testing.T) {
	_, ts := makeServer(t)
	statusCode, body := makeRequest(t, ts, http.MethodGet, "/debug/pprof/goroutine", "")
	require.Equal(t, http.StatusOK, statusCode)
	require.NotEmpty(t, body)
}

func makeServer(t *testing.T) (*mockLeveler, *httptest.Server) {
	leveler := &mockLeveler{}
	logger := slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))

	mux := http.NewServeMux()
	RegisterProfilingHandlers(mux, logger)
	RegisterLogLevelHandlers(mux, logger, leveler)

	ts := httptest.NewServer(mux)
	t.Cleanup(func() { ts.Close() })

	return leveler, ts
}

func makeRequest(t *testing.T, ts *httptest.Server, method, path, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBufferString(body))
	require.NoError(t, err)

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(respBody)
}
