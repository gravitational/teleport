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
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/trace"
)

func TestReadLimitedRequestBody(t *testing.T) {
	for name, tc := range map[string]struct {
		body          string
		contentLength int64
		maxSize       int64
		expectedBody  string
		expectedErr   require.ErrorAssertionFunc
	}{
		"known content length over limit": {
			body:          "123456",
			contentLength: 6,
			maxSize:       5,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsBadParameter(err), i...)
			},
		},
		"unknown content length over limit": {
			body:          "123456",
			contentLength: -1,
			maxSize:       5,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsBadParameter(err), i...)
			},
		},
		"exact limit": {
			body:          "12345",
			contentLength: -1,
			maxSize:       5,
			expectedBody:  "12345",
			expectedErr:   require.NoError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(tc.body))
			req.ContentLength = tc.contentLength

			handler := newTestHandler(t)
			body, err := handler.readLimitedRequestBody(req, tc.maxSize)
			tc.expectedErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tc.expectedBody, body.String())
		})
	}
}

// streamRecorder adapts an apievents.Stream to an events.SessionRecorder
// by adding a no-op io.Writer (required by the interface).
type streamRecorder struct {
	apievents.Stream
}

func (s *streamRecorder) Write(p []byte) (int, error) { return len(p), nil }

// newTestAudit creates a common.Audit that calls onRecord for each recorded event.
func newTestAudit(t *testing.T, onRecord func(apievents.PreparedSessionEvent)) common.Audit {
	t.Helper()
	streamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
		Inner: events.NewDiscardStreamer(),
		OnRecordEvent: func(_ context.Context, _ session.ID, pe apievents.PreparedSessionEvent) error {
			onRecord(pe)
			return nil
		},
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(t.Context(), "test-session")
	require.NoError(t, err)
	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  events.NewDiscardEmitter(),
		Recorder: events.WithNoOpPreparer(&streamRecorder{stream}),
	})
	require.NoError(t, err)
	return audit
}

// newTestApp creates a types.Application configured with the given LLM options.
func newTestApp(t *testing.T, llm *types.LLM) types.Application {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name: "test-llm-app",
	}, types.AppSpecV3{
		LLM: llm,
	})
	require.NoError(t, err)
	return app
}

// newTestSessionRequest creates an *http.Request with a SessionContext attached.
func newTestSessionRequest(t *testing.T, method, path string, body io.Reader, sessionCtx *common.SessionContext) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	return common.WithSessionContext(req, sessionCtx)
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := NewHandler(t.Context(), HandlerConfig{
		Log: slog.Default(),
	})
	require.NoError(t, err)
	return h
}
