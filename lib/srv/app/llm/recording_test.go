// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package llm

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// newCaptureRecorder returns a capturing recorder alongside a no-op
// SessionPreparerRecorder wrapping it, mirroring the pattern used by
// lib/httplib/httprecorder's own tests.
func newCaptureRecorder() (*eventstest.MockRecorderEmitter, events.SessionPreparerRecorder) {
	c := &eventstest.MockRecorderEmitter{}
	return c, events.WithNoOpPreparer(c)
}

// newRecordingTestApp returns a minimal types.Application suitable for
// exercising recordSessionExchange's recorder wiring.
func newRecordingTestApp(t *testing.T) types.Application {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name: "test-app",
	}, types.AppSpecV3{
		URI:        "http://localhost:1234",
		PublicAddr: "test-app.example.com",
	})
	require.NoError(t, err)
	return app
}

// recordExchange drives recordSessionExchange with sessionCtx injected into
// the request context, the way the LLM handler receives it. enabled mirrors the
// resolved HandlerConfig.LLMRecordingEnabled parameter, so tests can toggle
// recording without touching the process environment (keeping them parallel).
func recordExchange(enabled bool, w http.ResponseWriter, r *http.Request, sessionCtx *common.SessionContext, next http.Handler) error {
	return maybeRecordSessionExchange(slog.Default(), enabled, w, common.WithSessionContext(r, sessionCtx), next)
}

// newRecordingSessionCtx builds a SessionContext whose audit logger exposes rec
// as its session chunk recorder, mirroring how the app connections handler
// wires the chunk recorder into the session context (via the audit logger).
func newRecordingSessionCtx(t *testing.T, app types.Application, identity *tlsca.Identity, rec events.SessionPreparerRecorder) *common.SessionContext {
	t.Helper()
	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  events.NewDiscardEmitter(),
		Recorder: rec,
	})
	require.NoError(t, err)
	return &common.SessionContext{
		Identity: identity,
		App:      app,
		ChunkID:  "chunk-1",
		Audit:    audit,
	}
}

// TestHTTPRecordBodyChunks drives an app-session HTTP exchange through
// recordSessionExchange (the recording wrapper the LLM handler uses) and
// asserts that the request/response envelopes and body chunks are recorded,
// and that a response body larger than 64KB is persisted intact by the
// chunking in lib/httplib/httprecorder.
func TestHTTPRecordBodyChunks(t *testing.T) {
	t.Parallel()

	mock, rec := newCaptureRecorder()
	app := newRecordingTestApp(t)
	identity := &tlsca.Identity{Username: "alice"}

	sessionCtx := newRecordingSessionCtx(t, app, identity, rec)

	const reqBodySize = 10 * 1024
	reqBody := bytes.Repeat([]byte("r"), reqBodySize)

	// Larger than the per-chunk cap so the body is split across multiple
	// chunk events; the assertion below verifies the reconstructed body still
	// matches byte for byte.
	const respBodySize = 128 * 1024
	respBody := bytes.Repeat([]byte("x"), respBodySize)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, reqBody, got)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(respBody)
		require.NoError(t, err)
	})

	req := httptest.NewRequest(http.MethodPost, "/some/path", bytes.NewReader(reqBody))
	rw := httptest.NewRecorder()

	err := recordExchange(true /* enabled */, rw, req, sessionCtx, handler)
	require.NoError(t, err)
	require.Equal(t, respBody, rw.Body.Bytes())

	var (
		gotRequest     *apievents.AppSessionHTTPRequest
		gotResponse    *apievents.AppSessionHTTPResponse
		requestChunks  [][]byte
		responseChunks [][]byte
		lastReqChunk   bool
		lastRespChunk  bool
	)

	for _, e := range mock.Events() {
		switch evt := e.(type) {
		case *apievents.AppSessionHTTPRequest:
			gotRequest = evt
		case *apievents.AppSessionHTTPResponse:
			gotResponse = evt
		case *apievents.AppSessionHTTPRequestBodyChunk:
			requestChunks = append(requestChunks, evt.Data)
			if evt.IsLast {
				lastReqChunk = true
			}
		case *apievents.AppSessionHTTPResponseBodyChunk:
			responseChunks = append(responseChunks, evt.Data)
			if evt.IsLast {
				lastRespChunk = true
			}
		}
	}

	require.NotNil(t, gotRequest, "expected an AppSessionHTTPRequest event to be recorded")
	require.Equal(t, "alice", gotRequest.UserMetadata.User)
	require.Equal(t, "test-app", gotRequest.AppMetadata.AppName)
	require.Equal(t, http.MethodPost, gotRequest.Method)

	require.NotNil(t, gotResponse, "expected an AppSessionHTTPResponse event to be recorded")
	require.EqualValues(t, http.StatusOK, gotResponse.StatusCode)

	require.True(t, lastReqChunk, "expected a terminal request body chunk event")
	require.True(t, lastRespChunk, "expected a terminal response body chunk event")

	require.Equal(t, reqBody, bytes.Join(requestChunks, nil))

	gotRespBody := bytes.Join(responseChunks, nil)
	require.Len(t, gotRespBody, respBodySize, "response body chunks must reconstruct the full >64KB body")
	require.Equal(t, respBody, gotRespBody)
}

// TestHTTPRecordSkipsWrappingWhenRecorderNil verifies that recordSessionExchange
// serves the request unwrapped when there is no session chunk recorder
// (e.g. the path used for TCP apps, which never create a chunk recorder).
func TestHTTPRecordSkipsWrappingWhenRecorderNil(t *testing.T) {
	t.Parallel()

	app := newRecordingTestApp(t)
	identity := &tlsca.Identity{Username: "alice"}
	sessionCtx := &common.SessionContext{
		Identity: identity,
		App:      app,
		ChunkID:  "chunk-1",
		// Audit intentionally left nil: no audit logger means no session chunk
		// recorder, so recording is skipped.
	}

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()

	err := recordExchange(true /* enabled */, rw, req, sessionCtx, handler)
	require.NoError(t, err)
	require.True(t, handlerCalled)
	require.Equal(t, "ok", rw.Body.String())
}

// TestHTTPRecordSkippedWhenGateDisabled verifies that even with a non-nil
// session chunk recorder, recordSessionExchange does NOT record when the beam
// LLM recording gate is off (the default outside a beam with recording
// enabled): the handler is served unwrapped and no HTTP events are emitted.
func TestHTTPRecordSkippedWhenGateDisabled(t *testing.T) {
	t.Parallel()

	mock, rec := newCaptureRecorder()
	app := newRecordingTestApp(t)
	sessionCtx := newRecordingSessionCtx(t, app, &tlsca.Identity{Username: "alice"}, rec)

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/some/path", bytes.NewReader([]byte("body")))
	rw := httptest.NewRecorder()

	require.NoError(t, recordExchange(false /* enabled */, rw, req, sessionCtx, handler))
	require.True(t, handlerCalled)
	require.Equal(t, "ok", rw.Body.String())
	require.Empty(t, mock.Events(), "no HTTP events should be recorded when the beam LLM recording gate is off")
}

// failingRecorder is a SessionRecorder whose RecordEvent always fails, used
// to verify recordSessionExchange fails closed.
type failingRecorder struct {
	eventstest.MockRecorderEmitter
}

func (f *failingRecorder) RecordEvent(context.Context, apievents.PreparedSessionEvent) error {
	return assert.AnError
}

// TestHTTPRecordFailsClosedWhenRecordingFails verifies that when the initial
// request-metadata event cannot be recorded, recordSessionExchange returns an
// error and never invokes the downstream handler - so the caller can fail the
// request instead of proxying unrecorded traffic.
func TestHTTPRecordFailsClosedWhenRecordingFails(t *testing.T) {
	t.Parallel()

	app := newRecordingTestApp(t)
	identity := &tlsca.Identity{Username: "alice"}

	sessionCtx := newRecordingSessionCtx(t, app, identity, events.WithNoOpPreparer(&failingRecorder{}))

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()

	err := recordExchange(true /* enabled */, rw, req, sessionCtx, handler)
	require.Error(t, err)
	require.False(t, handlerCalled, "handler must not run when the initial recording event fails")
}

// TestBeamLLMRecordingEnabled verifies the beam LLM recording gate: recording
// is enabled only when both the beam runtime and LLM recording env vars are
// truthy. The environment is supplied via an injected lookup so the cases run
// in parallel without mutating the process environment.
func TestBeamLLMRecordingEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		runtime   string
		recording string
		want      bool
	}{
		{name: "both enabled", runtime: "yes", recording: "yes", want: true},
		{name: "runtime only", runtime: "yes", recording: "", want: false},
		{name: "recording only", runtime: "", recording: "yes", want: false},
		{name: "neither", runtime: "", recording: "", want: false},
		{name: "runtime explicitly false", runtime: "false", recording: "yes", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(key string) string {
				switch key {
				case teleport.BeamsRuntimeEnvVar:
					return tc.runtime
				case teleport.BeamsLLMRecordingEnvVar:
					return tc.recording
				default:
					return ""
				}
			}
			require.Equal(t, tc.want, BeamLLMRecordingEnabled(getenv))
		})
	}
}
