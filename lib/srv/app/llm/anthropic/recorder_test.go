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

package anthropic

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	llmtesting "github.com/gravitational/teleport/lib/srv/app/llm/testing"
)

// successStream is a valid messages API SSE stream. Input tokens are reported
// on message_start and output tokens on message_delta.
var successStream = strings.Join([]string{
	"event: message_start\n" + `data: {"type":"message_start","message":{"id":"msg_123","type":"message","content":[],"usage":{"input_tokens":15}}}`,
	"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"Hello"}}`,
	"event: message_delta\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":15,"output_tokens":60}}`,
	"event: message_stop",
}, "\n\n")

// TestParseProviderError covers the Anthropic error type to Teleport error
// mapping.
func TestParseProviderError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		body        string
		expectedErr error
	}{
		"timeout":         {`{"error":{"type":"timeout_error","message":"slow"}}`, llmerrors.ErrTimeout},
		"invalid request": {`{"error":{"type":"invalid_request_error","message":"bad"}}`, llmerrors.ErrBadRequest},
		"authentication":  {`{"error":{"type":"authentication_error","message":"nope"}}`, llmerrors.ErrUnauthorized},
		"permission":      {`{"error":{"type":"permission_error","message":"nope"}}`, llmerrors.ErrUnauthorized},
		"rate limit":      {`{"error":{"type":"rate_limit_error","message":"slow down"}}`, llmerrors.ErrRejected},
		"overloaded":      {`{"error":{"type":"overloaded_error","message":"busy"}}`, llmerrors.ErrRejected},
		"billing":         {`{"error":{"type":"billing_error","message":"pay"}}`, llmerrors.ErrRejected},
		"not found":       {`{"error":{"type":"not_found_error","message":"missing"}}`, llmerrors.ErrUnsupported},
		"unknown type":    {`{"error":{"type":"some_new_error","message":"?"}}`, llmerrors.ErrUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			apiErr, err := parseProviderError([]byte(tc.body))
			require.NoError(t, err)
			require.ErrorIs(t, apiErr, tc.expectedErr)
		})
	}

	t.Run("malformed body returns parse error", func(t *testing.T) {
		t.Parallel()
		_, err := parseProviderError([]byte(`{"error":`))
		require.Error(t, err)
	})
}

// TestNewErrorMessage covers the Teleport error to Anthropic error type
// mapping.
func TestNewErrorMessage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		err          error
		expectedType string
	}{
		"timeout":      {llmerrors.ErrTimeout, errorTypeTimeoutError},
		"canceled":     {llmerrors.ErrCanceled, errorTypeTimeoutError},
		"bad request":  {llmerrors.ErrBadRequest, errorTypeInvalidRequestError},
		"unauthorized": {llmerrors.ErrUnauthorized, errorTypeAuthenticationError},
		"rejected":     {llmerrors.ErrRejected, errorTypeRateLimitError},
		"unsupported":  {llmerrors.ErrUnsupported, errorTypeNotFoundError},
		"unknown":      {llmerrors.ErrUnknown, errorTypeAPIError},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msg := newErrorMessage(tc.err)
			require.Equal(t, tc.expectedType, msg.Error.Type)
			require.Contains(t, msg.Error.Message, tc.err.Error())
		})
	}

	require.Nil(t, newErrorMessage(nil))
}

// TestEndpointParseUsage covers extraction of token usage from a non-streaming
// messages API response.
func TestEndpointParseUsage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		in, out, err := Endpoint{}.ParseUsage([]byte(`{"usage":{"input_tokens":15,"output_tokens":20}}`))
		require.NoError(t, err)
		require.Equal(t, 15, in)
		require.Equal(t, 20, out)
	})

	t.Run("malformed body", func(t *testing.T) {
		t.Parallel()
		_, _, err := Endpoint{}.ParseUsage([]byte(`{"usage":`))
		require.Error(t, err)
	})
}

// TestProcessSSEEvents covers the messages API SSE processing.
func TestProcessSSEEvents(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	for name, tc := range map[string]struct {
		stream              string
		expectedInputTokens int
		expectedOutput      int
		expectedErr         require.ErrorAssertionFunc
		expectedDownstream  func(t *testing.T, body string)
	}{
		"success extracts usage and forwards events": {
			stream:              successStream,
			expectedInputTokens: 15,
			expectedOutput:      60,
			expectedErr:         require.NoError,
			expectedDownstream: func(t *testing.T, body string) {
				require.Equal(t, successStream+"\n\n", body)
			},
		},
		"error event surfaces provider error": {
			stream: "event: error\n" + `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrRejected, i...)
			},
			expectedDownstream: func(t *testing.T, body string) {
				require.Contains(t, body, "Overloaded")
				require.Contains(t, body, llmerrors.ErrRejected.Error())
			},
		},
		"invalid error event surfaces bad response": {
			stream: "event: error\n" + `data: {"type":"error","error":`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrBadResponse, i...)
			},
			expectedDownstream: func(t *testing.T, body string) {
				require.Contains(t, body, llmerrors.ErrBadResponse.Error())
			},
		},
		"malformed message_start is skipped": {
			stream:      "event: message_start\n" + `data: {"type":"message_start","message":`,
			expectedErr: require.NoError,
			expectedDownstream: func(t *testing.T, body string) {
				// The malformed event is dropped (continue), so nothing is
				// forwarded downstream.
				require.Empty(t, body)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out strings.Builder
			in, out2, err := processSSEEvents(
				context.Background(),
				log,
				io.NopCloser(strings.NewReader(tc.stream)),
				&out,
			)
			tc.expectedErr(t, err)
			require.Equal(t, tc.expectedInputTokens, in)
			require.Equal(t, tc.expectedOutput, out2)
			tc.expectedDownstream(t, out.String())
		})
	}
}

// TestProcessSSEEventsWriteFailure covers the error-event branch when the
// downstream write fails: the recorder must still surface the semantic error.
func TestProcessSSEEventsWriteFailure(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	for name, tc := range map[string]struct {
		stream      string
		expectedErr require.ErrorAssertionFunc
	}{
		"write error surfaces provider error": {
			stream: "event: error\n" + `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrRejected, i...)
			},
		},
		"invalid error event surfaces bad response": {
			stream: "event: error\n" + `data: {"type":"error","error":`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrBadResponse, i...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := llmtesting.NewFailingResponseWriter("text/event-stream")
			_, _, err := processSSEEvents(
				context.Background(),
				log,
				io.NopCloser(strings.NewReader(tc.stream)),
				w,
			)
			tc.expectedErr(t, err)
		})
	}
}

// buildResponseBody returns a valid non-streaming messages API response whose
// total size is roughly fillerBytes.
func buildResponseBody(fillerBytes int) []byte {
	text := strings.Repeat("A", fillerBytes)
	return fmt.Appendf(nil,
		`{"id":"msg_123","type":"message","content":[{"type":"text","text":%q}],"usage":{"input_tokens":15,"output_tokens":20}}`,
		text,
	)
}

// buildStreamEvents returns valid messages API SSE events.
func buildStreamEvents(numEvents int) [][]byte {
	events := make([][]byte, 0, numEvents+3)
	events = append(events, []byte("event: message_start\n"+`data: {"type":"message_start","message":{"id":"msg_123","type":"message","content":[],"usage":{"input_tokens":15}}}`+"\n\n"))
	for range numEvents {
		events = append(events, []byte("event: content_block_delta\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello world"}}`+"\n\n"))
	}
	events = append(events, []byte("event: message_delta\n"+`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":15,"output_tokens":60}}`+"\n\n"))
	events = append(events, []byte("event: message_stop"))
	return events
}

// BenchmarkResponseRecorderJSON tracks the non-streaming path.
//
//	go test ./lib/srv/app/llm/anthropic/ -run '^$' -bench BenchmarkResponseRecorderJSON -benchmem
func BenchmarkResponseRecorderJSON(b *testing.B) {
	log := slog.New(slog.DiscardHandler)
	for _, bc := range []struct {
		name string
		body []byte
	}{
		{"small", buildResponseBody(16)},
		{"medium_32KB", buildResponseBody(32 * 1024)},
		{"large_1MB", buildResponseBody(1024 * 1024)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.SetBytes(int64(len(bc.body)))
			b.ReportAllocs()

			for b.Loop() {
				w := llmtesting.NewDiscardResponseWriter("application/json")
				rec, err := NewResponseRecorder(log, w)
				require.NoError(b, err)
				rec.WriteHeader(http.StatusOK)
				_, err = rec.Write(bc.body)
				require.NoError(b, err)
				require.NoError(b, rec.Close())
			}
		})
	}
}

// BenchmarkResponseRecorderStream issues one Write per SSE event.
//
//	go test ./lib/srv/app/llm/anthropic/ -run '^$' -bench BenchmarkResponseRecorderStream -benchmem
func BenchmarkResponseRecorderStream(b *testing.B) {
	log := slog.New(slog.DiscardHandler)
	for _, bc := range []struct {
		name    string
		nEvents int
	}{
		{"32_events", 32},
		{"256_events", 256},
		{"1024_events", 1024},
	} {
		events := buildStreamEvents(bc.nEvents)
		var total int
		for _, e := range events {
			total += len(e)
		}
		b.Run(bc.name, func(b *testing.B) {
			b.SetBytes(int64(total))
			b.ReportAllocs()

			for b.Loop() {
				w := llmtesting.NewDiscardResponseWriter("text/event-stream")
				rec, err := NewResponseRecorder(log, w)
				require.NoError(b, err)
				rec.WriteHeader(http.StatusOK)
				for _, e := range events {
					_, err := rec.Write(e)
					require.NoError(b, err)
				}
				// Close waits for the SSE-processing goroutine to drain.
				require.NoError(b, rec.Close())
			}
		})
	}
}
