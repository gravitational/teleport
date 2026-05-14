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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestOpenAIStreamUsageReport asserts how streamOpenAISSEEvents extracts usage
// from the OpenAI Responses SSE protocol and forwards it to the reporter.
func TestOpenAIStreamUsageReport(t *testing.T) {
	const (
		createdEvent = "event: response.created\n" +
			`data: {"type":"response.created","response":{"id":"resp_1"}}` + "\n\n"
		completedEvent = "event: response.completed\n" +
			`data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":42,"output_tokens":25}}}` + "\n\n"
		completedNoUsage = "event: response.completed\n" +
			`data: {"type":"response.completed","response":{"id":"resp_1"}}` + "\n\n"
		completedMalformed = "event: response.completed\n" +
			`data: {"type":"response.completed","response":"not-an-object"}` + "\n\n"
		failedEvent = "event: response.failed\n" +
			`data: {"error":{"type":"server_error","message":"upstream failure"}}` + "\n\n"
	)

	for name, tc := range map[string]struct {
		input         string
		expectErr     require.ErrorAssertionFunc
		expectCalls   int
		expectedUsage usageReport
	}{
		"response.completed reports usage once with OpenAI format": {
			input:         createdEvent + completedEvent,
			expectErr:     require.NoError,
			expectCalls:   1,
			expectedUsage: usageReport{InputTokens: 42, OutputTokens: 25},
		},
		"response.failed before completion does not report usage": {
			input:       createdEvent + failedEvent,
			expectErr:   require.Error,
			expectCalls: 0,
		},
		"stream without response.completed does not report usage": {
			input:       createdEvent,
			expectErr:   require.NoError,
			expectCalls: 0,
		},
		"empty stream does not report usage": {
			input:       "",
			expectErr:   require.NoError,
			expectCalls: 0,
		},
		"response.completed without usage key reports zero": {
			input:         createdEvent + completedNoUsage,
			expectErr:     require.NoError,
			expectCalls:   1,
			expectedUsage: usageReport{},
		},
		"malformed response payload does not panic and reports zero": {
			input:         createdEvent + completedMalformed,
			expectErr:     require.NoError,
			expectCalls:   1,
			expectedUsage: usageReport{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:   io.NopCloser(strings.NewReader(tc.input)),
			}
			reporter := &recordingReporter{}
			w := httptest.NewRecorder()

			_, err := streamOpenAIResponsesSSEEvents(t.Context(), slog.Default(), w, resp, reporter)
			tc.expectErr(t, err)

			calls := reporter.snapshot()
			require.Len(t, calls, tc.expectCalls)
			if tc.expectCalls == 0 {
				return
			}
			require.Equal(t, types.LLMFormatOpenAI, calls[0].format,
				"usage must be reported under the OpenAI format, not Anthropic")
			require.Equal(t, tc.expectedUsage, calls[0].usage)
		})
	}

	t.Run("reporter error is swallowed and does not fail the stream", func(t *testing.T) {
		body := "event: response.completed\n" +
			`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2}}}` + "\n\n"
		resp := &http.Response{
			Header: http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:   io.NopCloser(strings.NewReader(body)),
		}
		reporter := &recordingReporter{err: errors.New("boom"), errOnce: true}
		w := httptest.NewRecorder()

		_, err := streamOpenAIResponsesSSEEvents(t.Context(), slog.Default(), w, resp, reporter)
		require.NoError(t, err, "reporter errors must be logged, not surfaced to the client")
		require.Len(t, reporter.snapshot(), 1)
	})
}
