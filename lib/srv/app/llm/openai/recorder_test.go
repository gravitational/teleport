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

package openai

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

// responsesSuccessStream is a valid responses API SSE stream. Usage is only
// reported on the terminal response.completed event.
var responsesSuccessStream = strings.Join([]string{
	"event: response.created\n" + `data: {"type":"response.created","response":{"id":"resp_123","object":"response"},"sequence_number":0}`,
	"event: response.output_text.delta\n" + `data: {"type":"response.output_text.delta","delta":"Hello","sequence_number":1}`,
	"event: response.output_text.delta\n" + `data: {"type":"response.output_text.delta","delta":"World","sequence_number":2}`,
	"event: response.completed\n" + `data: {"type":"response.completed","response":{"usage":{"input_tokens":15,"output_tokens":60,"total_tokens":75}},"sequence_number":3}`,
}, "\n\n")

// responsesIncompleteStream matches the documented incomplete response shape.
var responsesIncompleteStream = "event: response.incomplete\n" +
	`data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_tokens"},"usage":{"input_tokens":15,"output_tokens":60,"total_tokens":75},"sequence_number":1}}`

// TestParseProviderError covers the OpenAI status-code to Teleport error
// mapping.
func TestParseProviderError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		statusCode  int
		body        string
		expectedErr error
	}{
		"bad request":         {http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"bad"}}`, llmerrors.ErrBadRequest},
		"unauthorized":        {http.StatusUnauthorized, `{"error":{"type":"authentication_error","message":"nope"}}`, llmerrors.ErrUnauthorized},
		"forbidden":           {http.StatusForbidden, `{"error":{"type":"permission_error","message":"nope"}}`, llmerrors.ErrUnauthorized},
		"rate limit":          {http.StatusTooManyRequests, `{"error":{"type":"rate_limit_exceeded","message":"slow down"}}`, llmerrors.ErrRejected},
		"service unavailable": {http.StatusServiceUnavailable, `{"error":{"type":"server_error","message":"busy"}}`, llmerrors.ErrRejected},
		"unknown status":      {http.StatusInternalServerError, `{"error":{"type":"server_error","message":"?"}}`, llmerrors.ErrUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			apiErr, err := parseProviderError(tc.statusCode, []byte(tc.body))
			require.NoError(t, err)
			require.ErrorIs(t, apiErr, tc.expectedErr)
		})
	}

	t.Run("malformed body returns parse error", func(t *testing.T) {
		t.Parallel()
		_, err := parseProviderError(http.StatusBadRequest, []byte(`{"error":`))
		require.Error(t, err)
	})
}

// TestNewErrorMessage covers the Teleport error to OpenAI error type mapping.
func TestNewErrorMessage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		err          error
		expectedType string
	}{
		"bad request":  {llmerrors.ErrBadRequest, errorTypeInvalidRequest},
		"unauthorized": {llmerrors.ErrUnauthorized, errorTypeInvalidRequest},
		"unsupported":  {llmerrors.ErrUnsupported, errorTypeInvalidRequest},
		"rejected":     {llmerrors.ErrRejected, errorTypeRateLimitExceeded},
		"timeout":      {llmerrors.ErrTimeout, errorTypeServerError},
		"unknown":      {llmerrors.ErrUnknown, errorTypeServerError},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msg := newErrorEnvelope(tc.err)
			require.Equal(t, tc.expectedType, msg.Error.Type)
			require.Contains(t, msg.Error.Message, tc.err.Error())
		})
	}

	require.Nil(t, newErrorEnvelope(nil))
}

func TestEndpointParseUsage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		endpointType         endpointType
		body                 string
		expectedInputTokens  int
		expectedOutputTokens int
		expectedErr          require.ErrorAssertionFunc
	}{
		"responses success": {
			endpointType:         endpointTypeResponses,
			body:                 `{"usage":{"input_tokens":15,"output_tokens":20,"total_tokens":35}}`,
			expectedInputTokens:  15,
			expectedOutputTokens: 20,
			expectedErr:          require.NoError,
		},
		"responses malformed body": {
			endpointType: endpointTypeResponses,
			body:         `{"usage":`,
			expectedErr:  require.Error,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in, out, err := Endpoint{endpointType: tc.endpointType}.ParseUsage([]byte(tc.body))
			tc.expectedErr(t, err)
			require.Equal(t, tc.expectedInputTokens, in)
			require.Equal(t, tc.expectedOutputTokens, out)
		})
	}
}

func TestEndpointProcessSSE(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	for name, tc := range map[string]struct {
		endpointType        endpointType
		stream              string
		expectedInputTokens int
		expectedOutput      int
		expectedErr         require.ErrorAssertionFunc
		expectedDownstream  func(t *testing.T, body string)
	}{
		"responses success extracts usage and forwards events": {
			endpointType:        endpointTypeResponses,
			stream:              responsesSuccessStream,
			expectedInputTokens: 15,
			expectedOutput:      60,
			expectedErr:         require.NoError,
			expectedDownstream: func(t *testing.T, body string) {
				require.Equal(t, responsesSuccessStream+"\n\n", body)
			},
		},
		"responses missing end event forwards events and reports error": {
			endpointType: endpointTypeResponses,
			stream: strings.Join([]string{
				"event: response.created\n" + `data: {"type":"response.created","response":{"id":"resp_123","object":"response"},"sequence_number":0}`,
				"event: response.output_text.delta\n" + `data: {"type":"response.output_text.delta","delta":"Hello","sequence_number":1}`,
			}, "\n\n"),
			expectedErr: require.Error,
			expectedDownstream: func(t *testing.T, body string) {
				require.NotEmpty(t, body)
			},
		},
		"responses incomplete event extracts usage and forwards events": {
			endpointType:        endpointTypeResponses,
			stream:              responsesIncompleteStream,
			expectedInputTokens: 15,
			expectedOutput:      60,
			expectedErr:         require.NoError,
			expectedDownstream: func(t *testing.T, body string) {
				require.Equal(t, responsesIncompleteStream+"\n\n", body)
			},
		},
		"responses response.failed surfaces provider error": {
			endpointType: endpointTypeResponses,
			stream:       "event: response.failed\n" + `data: {"type":"response.failed","response":{"error":{"type":"server_error","message":"the model failed"}},"sequence_number":5}`,
			expectedErr:  require.Error,
			expectedDownstream: func(t *testing.T, body string) {
				require.Equal(
					t,
					"event: response.failed\n"+`data: {"type":"response.failed","response":{"error":{"type":"server_error","message":"`+llmerrors.ErrUnknown.Error()+`: the model failed"}},"sequence_number":5}`+"\n\n",
					body,
					"expected the exact same SSE event shape with Teleport error, but got a different result",
				)
			},
		},
		"responses error surfaces provider error": {
			endpointType: endpointTypeResponses,
			stream:       "event: error\n" + `data: {"type":"error","message":"the model failed","sequence_number":5}`,
			expectedErr:  require.Error,
			expectedDownstream: func(t *testing.T, body string) {
				require.Equal(
					t,
					"event: error\n"+`data: {"type":"error","message":"`+llmerrors.ErrUnknown.Error()+`: the model failed","sequence_number":5}`+"\n\n",
					body,
					"expected the exact same SSE event shape with Teleport error, but got a different result",
				)
			},
		},
		"responses invalid response.failed surfaces bad response": {
			endpointType: endpointTypeResponses,
			stream:       "event: response.failed\n" + `data: {"type":"response.failed","response":`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrBadResponse, i...)
			},
			expectedDownstream: func(t *testing.T, body string) {
				require.Contains(t, body, llmerrors.ErrBadResponse.Error())
			},
		},
		"responses malformed completed event surfaces bad response": {
			endpointType: endpointTypeResponses,
			stream:       "event: response.completed\n" + `data: {"type":"response.completed","response":`,
			expectedErr:  require.Error,
			expectedDownstream: func(t *testing.T, body string) {
				require.Empty(t, body)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out strings.Builder
			in, out2, err := Endpoint{endpointType: tc.endpointType}.ProcessSSE(
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

// TestEndpointProcessSSEWriteFailure covers the error-event branch when the
// downstream write fails: the recorder must still surface the semantic error.
func TestEndpointProcessSSEWriteFailure(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	for name, tc := range map[string]struct {
		endpointType endpointType
		stream       string
		expectedErr  require.ErrorAssertionFunc
	}{
		"responses.failed event error surfaces to downstream": {
			endpointType: endpointTypeResponses,
			stream:       "event: response.failed\n" + `data: {"type":"response.failed","response":{"error":{"type":"server_error","message":"the model failed"}}}`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "the model failed")
			},
		},
		"responses.failed error event surfaces to downstream as bad response": {
			endpointType: endpointTypeResponses,
			stream:       "event: response.failed\n" + `data: {"type":"response.failed","response":`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrBadResponse, i...)
			},
		},
		"responses error event surfaces to downstream": {
			endpointType: endpointTypeResponses,
			stream:       "event: error\n" + `data: {"type":"server_error","message":"the model failed"}`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "the model failed")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := llmtesting.NewFailingResponseWriter("text/event-stream")
			_, _, err := Endpoint{endpointType: tc.endpointType}.ProcessSSE(
				context.Background(),
				log,
				io.NopCloser(strings.NewReader(tc.stream)),
				w,
			)
			tc.expectedErr(t, err)
		})
	}
}

// buildResponsesBody returns a valid non-streaming responses API response whose
// total size is roughly fillerBytes.
func buildResponsesBody(fillerBytes int) []byte {
	text := strings.Repeat("A", fillerBytes)
	return fmt.Appendf(nil,
		`{"id":"resp_123","object":"response","output":[{"type":"message","content":[{"type":"output_text","text":%q}]}],"usage":{"input_tokens":15,"output_tokens":20,"total_tokens":35}}`,
		text,
	)
}

// buildResponsesStreamEvents returns valid responses API SSE events.
func buildResponsesStreamEvents(numEvents int) [][]byte {
	events := make([][]byte, 0, numEvents+2)
	events = append(events, []byte("event: response.created\n"+`data: {"type":"response.created","response":{"id":"resp_123","object":"response"}}`+"\n\n"))
	for range numEvents {
		events = append(events, []byte("event: response.output_text.delta\n"+`data: {"type":"response.output_text.delta","delta":"Hello world"}`+"\n\n"))
	}
	events = append(events, []byte("event: response.completed\n"+`data: {"type":"response.completed","response":{"usage":{"input_tokens":15,"output_tokens":60,"total_tokens":75}}}`+"\n\n"))
	return events
}

// BenchmarkResponseRecorderJSON tracks the non-streaming path for each API
// endpoint format.
//
//	go test ./lib/srv/app/llm/openai/ -run '^$' -bench BenchmarkResponseRecorderJSON -benchmem
func BenchmarkResponseRecorderJSON(b *testing.B) {
	log := slog.New(slog.DiscardHandler)
	for _, bc := range []struct {
		name string
		ep   endpointType
		body []byte
	}{
		{"responses/small", endpointTypeResponses, buildResponsesBody(16)},
		{"responses/medium_32KB", endpointTypeResponses, buildResponsesBody(32 * 1024)},
		{"responses/large_1MB", endpointTypeResponses, buildResponsesBody(1024 * 1024)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.SetBytes(int64(len(bc.body)))
			b.ReportAllocs()

			for b.Loop() {
				w := llmtesting.NewDiscardResponseWriter("application/json")
				rec, err := NewResponseRecorder(log, &RequestInfo{endpointType: bc.ep}, w)
				require.NoError(b, err)
				rec.WriteHeader(http.StatusOK)
				_, err = rec.Write(bc.body)
				require.NoError(b, err)
				require.NoError(b, rec.Close())
			}
		})
	}
}

// BenchmarkResponseRecorderStream issues one Write per SSE event for each API
// endpoint format.
//
//	go test ./lib/srv/app/llm/openai/ -run '^$' -bench BenchmarkResponseRecorderStream -benchmem
func BenchmarkResponseRecorderStream(b *testing.B) {
	log := slog.New(slog.DiscardHandler)
	for _, bc := range []struct {
		name   string
		ep     endpointType
		events [][]byte
	}{
		{"responses/32_events", endpointTypeResponses, buildResponsesStreamEvents(32)},
		{"responses/256_events", endpointTypeResponses, buildResponsesStreamEvents(256)},
		{"responses/1024_events", endpointTypeResponses, buildResponsesStreamEvents(1024)},
	} {
		var total int
		for _, e := range bc.events {
			total += len(e)
		}
		b.Run(bc.name, func(b *testing.B) {
			b.SetBytes(int64(total))
			b.ReportAllocs()

			for b.Loop() {
				w := llmtesting.NewDiscardResponseWriter("text/event-stream")
				rec, err := NewResponseRecorder(log, &RequestInfo{endpointType: bc.ep}, w)
				require.NoError(b, err)
				rec.WriteHeader(http.StatusOK)
				for _, e := range bc.events {
					_, err := rec.Write(e)
					require.NoError(b, err)
				}
				// Close waits for the SSE-processing goroutine to drain.
				require.NoError(b, rec.Close())
			}
		})
	}
}
