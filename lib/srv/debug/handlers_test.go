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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestLogLevel(t *testing.T) {
	cfg, ts := makeServer()
	defer ts.Close()

	statusCode, logLevel := makeRequest(t, ts, GetLogLevelMethod, LogLevelEndpoint, "")
	require.Equal(t, http.StatusOK, statusCode)
	require.Equal(t, cfg.LoggerLevel.Level().String(), logLevel)

	// Invalid log level
	statusCode, _ = makeRequest(t, ts, http.MethodPut, LogLevelEndpoint, "RANDOM")
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)

	for _, logLevel := range logutils.SupportedLevelsText {
		t.Run("Set"+logLevel, func(t *testing.T) {
			statusCode, _ := makeRequest(t, ts, http.MethodPut, LogLevelEndpoint, logLevel)
			require.Equal(t, http.StatusOK, statusCode)

			statusCode, retrievedLogLevel := makeRequest(t, ts, GetLogLevelMethod, LogLevelEndpoint, "")
			require.Equal(t, http.StatusOK, statusCode)
			require.Equal(t, logLevel, retrievedLogLevel)
		})

		t.Run("SetLower"+logLevel, func(t *testing.T) {
			statusCode, _ := makeRequest(t, ts, http.MethodPut, LogLevelEndpoint, strings.ToLower(logLevel))
			require.Equal(t, http.StatusOK, statusCode)

			statusCode, retrievedLogLevel := makeRequest(t, ts, GetLogLevelMethod, LogLevelEndpoint, "")
			require.Equal(t, http.StatusOK, statusCode)
			require.Equal(t, logLevel, retrievedLogLevel)
		})
	}
}

func TestCollectProfiles(t *testing.T) {
	_, ts := makeServer()
	defer ts.Close()

	statusCode, body := makeRequest(t, ts, http.MethodGet, "/debug/pprof/goroutine", "")
	require.Equal(t, http.StatusOK, statusCode)
	require.NotEmpty(t, body)
}

func makeServer() (*servicecfg.Config, *httptest.Server) {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.DebugService.Enabled = true

	// Define the logger to avoid modifying global loggers.
	cfg.Log = logrus.New()
	cfg.Logger = slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))

	return cfg, httptest.NewServer(NewServeMux(cfg.Logger, cfg))
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
