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
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestLogLevel(t *testing.T) {
	cfg, ts := makeServer()
	defer ts.Close()
	httpClient := ts.Client()

	require.Equal(t, cfg.LoggerLevel.Level().String(), retrieveLogLevel(t, ts.URL, httpClient))

	// Invalid log level
	resp, err := httpClient.Do(makeRequest(t, ts.URL, http.MethodPut, LogLevelEndpoint, "RANDOM"))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	for _, logLevel := range logutils.SupportedLevelsText {
		t.Run("Set"+logLevel, func(t *testing.T) {
			resp, err = httpClient.Do(makeRequest(t, ts.URL, http.MethodPut, LogLevelEndpoint, logLevel))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, logLevel, retrieveLogLevel(t, ts.URL, httpClient))
		})
	}
}

func TestCollectProfiles(t *testing.T) {
	_, ts := makeServer()
	defer ts.Close()
	httpClient := ts.Client()

	resp, err := httpClient.Do(makeRequest(t, ts.URL, http.MethodGet, "/debug/pprof/goroutine", ""))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, respBody)
}

func retrieveLogLevel(t *testing.T, url string, httpClient *http.Client) string {
	t.Helper()

	resp, err := httpClient.Do(makeRequest(t, url, GetLogLevelMethod, LogLevelEndpoint, ""))
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(respBody)
}

func makeServer() (*servicecfg.Config, *httptest.Server) {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.DebugService.Enabled = true

	// Define the logger to avoid modifying global loggers.
	cfg.Log = logrus.New()
	cfg.Logger = slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))

	return cfg, httptest.NewServer(NewServeMux(context.Background(), cfg.Logger, cfg))
}

func makeRequest(t *testing.T, url, method, path, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	return req
}
