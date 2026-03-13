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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
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

	svc := NewService(ServiceConfig{
		Logger:  logger,
		Leveler: leveler,
	})

	mux := http.NewServeMux()
	RegisterProfilingHandlers(mux, logger)
	RegisterLogLevelHandlers(mux, svc)

	ts := httptest.NewServer(mux)
	t.Cleanup(func() { ts.Close() })

	return leveler, ts
}

func TestLogStreamNotRegisteredWithoutBroadcaster(t *testing.T) {
	// When no broadcaster is registered, /log-stream should not be available.
	// This verifies the gating: when debug_service.enabled=false, setting
	// logBroadcaster to nil prevents the endpoint from being exposed.
	logger := slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))

	svc := NewService(ServiceConfig{
		Logger:  logger,
		Leveler: &mockLeveler{},
	})

	mux := http.NewServeMux()
	RegisterLogLevelHandlers(mux, svc)
	// Deliberately NOT calling RegisterLogStreamHandler.

	ts := httptest.NewServer(mux)
	defer ts.Close()

	statusCode, _ := makeRequest(t, ts, http.MethodGet, "/log-stream", "")
	require.Equal(t, http.StatusNotFound, statusCode, "/log-stream should return 404 when broadcaster is not registered")
}

func TestLogStreamRegisteredWithBroadcaster(t *testing.T) {
	// When a broadcaster IS registered, /log-stream should be available.
	logger := slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))
	broadcaster := logutils.NewLogBroadcaster()

	svc := NewService(ServiceConfig{
		Logger:      logger,
		Leveler:     &mockLeveler{},
		Broadcaster: broadcaster,
	})

	mux := http.NewServeMux()
	RegisterLogStreamHandler(mux, svc)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Can't use makeRequest here because it calls io.ReadAll which blocks
	// on the streaming response. Just check the status code.
	resp, err := http.Get(ts.URL + "/log-stream")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "/log-stream should return 200 when broadcaster is registered")
}

func TestLogEntryToMap(t *testing.T) {
	ts := timestamppb.Now()
	entry := &debugpb.LogEntry{
		Timestamp: ts,
		Level:     "INFO",
		Message:   "hello world",
		Caller:    "main.go:42",
		Component: "auth",
		TraceId:   "abc123",
		SpanId:    "def456",
		Attributes: map[string]string{
			"user": "alice",
		},
	}

	m := logEntryToMap(entry)
	assert.Equal(t, "INFO", m["level"])
	assert.Equal(t, "hello world", m["message"])
	assert.Equal(t, "main.go:42", m["caller"])
	assert.Equal(t, "auth", m["component"])
	assert.Equal(t, "abc123", m["trace_id"])
	assert.Equal(t, "def456", m["span_id"])
	assert.Equal(t, "alice", m["user"])
	assert.Contains(t, m, "timestamp")
}

func TestLogEntryToMap_MinimalFields(t *testing.T) {
	entry := &debugpb.LogEntry{
		Level:   "DEBUG",
		Message: "simple",
	}

	m := logEntryToMap(entry)
	assert.Equal(t, "DEBUG", m["level"])
	assert.Equal(t, "simple", m["message"])
	// Optional fields should not be present.
	assert.NotContains(t, m, "caller")
	assert.NotContains(t, m, "component")
	assert.NotContains(t, m, "trace_id")
	assert.NotContains(t, m, "span_id")
	assert.NotContains(t, m, "timestamp")
}

func TestHandleLogStream_ReceivesEntries(t *testing.T) {
	logger := slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))
	broadcaster := logutils.NewLogBroadcaster()

	svc := NewService(ServiceConfig{
		Logger:      logger,
		Leveler:     &mockLeveler{},
		Broadcaster: broadcaster,
	})

	mux := http.NewServeMux()
	RegisterLogStreamHandler(mux, svc)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/log-stream?level=INFO", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Give subscription time to register.
	time.Sleep(50 * time.Millisecond)

	// Broadcast an entry.
	broadcaster.Broadcast(&debugpb.LogEntry{
		Level:   "INFO",
		Message: "stream-test-msg",
	}, slog.LevelInfo)

	// Read the NDJSON line.
	scanner := bufio.NewScanner(resp.Body)
	readCh := make(chan map[string]any, 1)
	go func() {
		if scanner.Scan() {
			var m map[string]any
			if json.Unmarshal(scanner.Bytes(), &m) == nil {
				readCh <- m
			}
		}
	}()

	select {
	case m := <-readCh:
		assert.Equal(t, "INFO", m["level"])
		assert.Equal(t, "stream-test-msg", m["message"])
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for log stream entry")
	}
}

func TestHandleLogStream_InvalidLevel(t *testing.T) {
	logger := slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{}))
	broadcaster := logutils.NewLogBroadcaster()

	svc := NewService(ServiceConfig{
		Logger:      logger,
		Leveler:     &mockLeveler{},
		Broadcaster: broadcaster,
	})

	mux := http.NewServeMux()
	RegisterLogStreamHandler(mux, svc)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	statusCode, body := makeRequest(t, ts, http.MethodGet, "/log-stream?level=BOGUS", "")
	assert.Equal(t, http.StatusBadRequest, statusCode)
	assert.Contains(t, body, "invalid level")
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
