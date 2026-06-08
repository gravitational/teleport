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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/httplib/sse"
	"github.com/gravitational/teleport/lib/itertools/stream"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
)

func TestHandleNonStreaming(t *testing.T) {
	expectStatus := func(statusCode int) require.ValueAssertionFunc {
		return func(tt require.TestingT, i1 any, i2 ...any) {
			require.Equal(tt, statusCode, i1, i2...)
		}
	}

	expectContentLength := func(length int) require.ValueAssertionFunc {
		return func(tt require.TestingT, i1 any, i2 ...any) {
			contentLength, err := strconv.Atoi(i1.(string))
			require.NoError(tt, err, "expected content length to not be empty")
			require.Equal(tt, length, contentLength, i2...)
		}
	}

	const successResponse = `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello"}],"usage":{"input_tokens":15, "output_tokens": 20}}`

	for name, tc := range map[string]struct {
		providerStatusCode           int
		providerBody                 string
		providerContentType          string
		expectedDownstreamBody       require.ValueAssertionFunc
		expectedDownstreamStatusCode require.ValueAssertionFunc
		expectedContentLength        require.ValueAssertionFunc
		expectedRecorder             require.ValueAssertionFunc
	}{
		"success": {
			providerStatusCode:  http.StatusOK,
			providerBody:        successResponse,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				// Expect unmodified response.
				require.Equal(tt, successResponse, body, i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			// Expect content length equal to the original message.
			expectedContentLength: expectContentLength(len(successResponse)),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, len(successResponse), rec.Written())
				require.Equal(tt, 15, rec.InputTokensCount(), i2...)
				require.Equal(tt, 20, rec.OutputTokensCount(), i2...)
				require.NoError(tt, rec.Err(), i2...)
			},
		},
		"non-json response": {
			providerStatusCode:  http.StatusOK,
			providerBody:        "Non-JSON result",
			providerContentType: "text/html",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				var apiErr errorEnvelope
				require.NoError(tt, json.Unmarshal([]byte(body), &apiErr), "expected message to be in JSON format")
				require.Contains(tt, apiErr.Error.Message, llmerrors.ErrBadResponse.Error(), "expected error response to include Teleport message")
				require.Contains(tt, apiErr.Error.Message, `unsupported "text/html" response type`, "expected error response to include the unsupported content-type detail")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusInternalServerError),
			// In case of errors, we always expect empty Content-Length.
			expectedContentLength: require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"api invalid request includes original message": {
			providerStatusCode:  http.StatusBadRequest,
			providerBody:        `{"error":{"type":"invalid_request_error","message":"invalid model"}}`,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				var apiErr errorEnvelope
				require.NoError(tt, json.Unmarshal([]byte(body), &apiErr), "expected message to be in JSON format")
				require.Contains(tt, apiErr.Error.Message, "invalid model", "expected error response to include original message")
				require.Contains(tt, apiErr.Error.Message, llmerrors.ErrBadRequest.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusBadRequest),
			// In case of errors, we always expect empty Content-Length.
			expectedContentLength: require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.Equal(tt, 0, rec.OutputTokensCount(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadRequest, i2...)
			},
		},
		"auth error 401 rewrites message": {
			providerStatusCode:  http.StatusUnauthorized,
			providerBody:        `{"error":{"type":"authentication_error","message":"original provider secret"}}`,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				var apiErr errorEnvelope
				require.NoError(tt, json.Unmarshal([]byte(body), &apiErr), "expected message to be in JSON format")
				require.NotContains(tt, body, "original provider secret", "expected error response to NOT include original message")
				require.Contains(tt, apiErr.Error.Message, llmerrors.ErrUnauthorized.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusUnauthorized),
			// In case of errors, we always expect empty Content-Length.
			expectedContentLength: require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrUnauthorized, i2...)
			},
		},
		"empty success response": {
			providerStatusCode:           http.StatusOK,
			providerBody:                 "",
			providerContentType:          "application/json",
			expectedDownstreamBody:       require.Empty,
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(0),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Empty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"empty error response": {
			providerStatusCode:  http.StatusUnauthorized,
			providerBody:        "",
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				var apiErr errorEnvelope
				require.NoError(tt, json.Unmarshal([]byte(body), &apiErr), "expected message to be in JSON format")
				require.Contains(tt, apiErr.Error.Message, llmerrors.ErrBadResponse.Error(), "expected error response to include Teleport message")
				require.Contains(tt, apiErr.Error.Message, "invalid JSON response", "expected error response to include the unparseable-body detail")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusUnauthorized),
			expectedContentLength:        require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"broken response": {
			providerStatusCode:  http.StatusOK,
			providerBody:        `{"id":"msg_123","type":"message","content"`,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				// Expect unmodified response.
				require.Equal(tt, `{"id":"msg_123","type":"message","content"`, body, i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			// Here, the original message is not parsed as error, and will be
			// forwarded as is to downstream.
			expectedContentLength: expectContentLength(42),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, 42, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"no write header success": {
			// WriteHeader is not called.
			providerStatusCode:  0,
			providerBody:        successResponse,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				// Expect unmodified response.
				require.Equal(tt, successResponse, body, i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(len(successResponse)),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, len(successResponse), rec.Written(), i2...)
				require.Equal(tt, 15, rec.InputTokensCount(), i2...)
				require.Equal(tt, 20, rec.OutputTokensCount(), i2...)
				require.NoError(tt, rec.Err(), i2...)
			},
		},
		"no write header unsupported content-type": {
			// WriteHeader is not called.
			providerStatusCode:  0,
			providerBody:        "Non-JSON result",
			providerContentType: "text/html",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				var apiErr errorEnvelope
				require.NoError(tt, json.Unmarshal([]byte(body), &apiErr), "expected message to be in JSON format")
				require.Contains(tt, apiErr.Error.Message, llmerrors.ErrBadResponse.Error(), "expected error response to include Teleport message")
				require.Contains(tt, apiErr.Error.Message, `unsupported "text/html" response type`, "expected error response to include the unsupported content-type detail")
			},
			// Without WriteHeader the recorder cannot force a 500 status nor
			// strip the original Content-Length, so the status stays 200.
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(len("Non-JSON result")),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// This scenario covers when the provider responses returns all
			// at once (single write). This is less realistic for large
			// responses.
			t.Run("single write", func(t *testing.T) {
				w := httptest.NewRecorder()
				rec, err := NewResponseRecorder(slog.Default(), w)
				require.NoError(t, err)

				rec.Header().Add("Content-Type", tc.providerContentType)
				rec.Header().Add("Content-Length", strconv.Itoa(len(tc.providerBody)))
				// A status code of 0 simulates a caller that never calls
				// WriteHeader, exercising the default 200 status path.
				if tc.providerStatusCode != 0 {
					rec.WriteHeader(tc.providerStatusCode)
				}
				if len(tc.providerBody) > 0 {
					_, err = io.WriteString(rec, tc.providerBody)
					require.NoError(t, err)
				}

				require.NoError(t, rec.Close())

				resp := w.Result()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				tc.expectedRecorder(t, rec)
				tc.expectedDownstreamStatusCode(t, resp.StatusCode)
				tc.expectedContentLength(t, w.Header().Get("Content-Length"))
				tc.expectedDownstreamBody(t, string(body))
			})
			// This scenario covers when the provider responses returns in
			// multiple writes. This is more realistic considering larger
			// responses and network conditions.
			t.Run("multi write", func(t *testing.T) {
				w := httptest.NewRecorder()
				rec, err := NewResponseRecorder(slog.Default(), w)
				require.NoError(t, err)

				rec.Header().Add("Content-Type", tc.providerContentType)
				rec.Header().Add("Content-Length", strconv.Itoa(len(tc.providerBody)))
				// A status code of 0 simulates a caller that never calls
				// WriteHeader, exercising the default 200 status path.
				if tc.providerStatusCode != 0 {
					rec.WriteHeader(tc.providerStatusCode)
				}
				for chunk := range slices.Chunk([]byte(tc.providerBody), 1) {
					_, err = rec.Write(chunk)
					require.NoError(t, err)
				}

				require.NoError(t, rec.Close())

				resp := w.Result()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				tc.expectedRecorder(t, rec)
				tc.expectedDownstreamStatusCode(t, resp.StatusCode)
				tc.expectedContentLength(t, w.Header().Get("Content-Length"))
				tc.expectedDownstreamBody(t, string(body))
			})
		})
	}
}

func TestHandleStreaming(t *testing.T) {
	expectStatus := func(statusCode int) require.ValueAssertionFunc {
		return func(tt require.TestingT, i1 any, i2 ...any) {
			require.Equal(tt, statusCode, i1, i2...)
		}
	}

	successStream := strings.Join([]string{
		"event: message_start\n" + `data: {"type": "message_start", "message":{"id":"msg_123","type":"message","content":[],"usage":{"input_tokens":15}}}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"Hello"}}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"World"}}`,
		"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":0}`,
		"event: message_delta\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null,"stop_details":null},"usage":{"input_tokens":15,"output_tokens":60}}`,
		"event: message_stop",
	}, "\n\n")

	for name, tc := range map[string]struct {
		providerResp                 func(w http.ResponseWriter)
		expectedDownstreamBody       require.ValueAssertionFunc
		expectedDownstreamStatusCode require.ValueAssertionFunc
		expectedRecorder             require.ValueAssertionFunc
	}{
		"success": {
			providerResp: func(w http.ResponseWriter) {
				w.Header().Add("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, successStream)
			},
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				// Expect unmodified response.
				require.Equal(tt, successStream+"\n\n", body, i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				// events length + last event line break.
				require.Equal(tt, len(successStream)+2, rec.Written())
				require.Equal(tt, 15, rec.InputTokensCount(), i2...)
				require.Equal(tt, 60, rec.OutputTokensCount(), i2...)
				require.NoError(tt, rec.Err())
			},
		},
		"with error": {
			providerResp: func(w http.ResponseWriter) {
				w.Header().Add("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, "event: error\n"+`data: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}`)
			},
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body, _ := i1.(string)
				evt := readSSEOneEvent(tt, body)
				var apiErr errorEnvelope
				require.Equal(t, "error", evt.Event)
				require.NoError(tt, json.Unmarshal(evt.Data, &apiErr), "expected message to be in JSON format")
				require.Contains(tt, body, "Overloaded", "expected error response to include original message")
				require.Contains(tt, body, llmerrors.ErrRejected.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, 198, rec.Written())
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrRejected)
			},
		},
		"empty stream": {
			providerResp: func(w http.ResponseWriter) {
				w.Header().Add("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
			},
			expectedDownstreamBody:       require.Empty,
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Empty(tt, rec.Written())
			},
		},
		"broken stream": {
			providerResp: func(w http.ResponseWriter) {
				w.Header().Add("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				// Broken JSON response.
				io.WriteString(w, "event: message_start\n"+`data: {"type": "message_start", "message":{"id":"msg_123","type":"message","content":`)
			},
			expectedDownstreamBody:       require.Empty,
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Empty(tt, rec.Written())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rec, err := NewResponseRecorder(slog.Default(), w)
			require.NoError(t, err)
			tc.providerResp(rec)
			require.NoError(t, rec.Close())

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			tc.expectedRecorder(t, rec)
			tc.expectedDownstreamStatusCode(t, resp.StatusCode)
			tc.expectedDownstreamBody(t, string(body))
		})
	}
}

// TestHandleStreamingErrorEventWriteFailure covers the error-event branch of
// processSSEEvents when the downstream write fails: the client connection is
// broken, but the recorder must still surface the semantic error via Err().
func TestHandleStreamingErrorEventWriteFailure(t *testing.T) {
	for name, tc := range map[string]struct {
		stream      string
		expectedErr require.ErrorAssertionFunc
	}{
		"write error surfaces provider error": {
			stream: "event: error\n" + `data: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrRejected, i...)
			},
		},
		"invalid error event surfaces bad response": {
			stream: "event: error\n" + `data: {"type": "error", "error":`,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, llmerrors.ErrBadResponse, i...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := &failingResponseWriter{header: http.Header{}}
			w.header.Set("Content-Type", "text/event-stream")

			rec, err := NewResponseRecorder(slog.Default(), w)
			require.NoError(t, err)

			rec.WriteHeader(http.StatusOK)
			_, err = io.WriteString(rec, tc.stream)
			require.NoError(t, err)
			// Close waits on the streaming goroutine, so Err() is populated.
			require.NoError(t, rec.Close())
			tc.expectedErr(t, rec.Err())
		})
	}
}

func readSSEOneEvent(t require.TestingT, str string) sse.Event {
	events, err := stream.Collect(sse.ReadEvents(strings.NewReader(str)))
	require.NoError(t, err)
	require.Len(t, events, 1)
	return events[0]
}

// discardResponseWriter is a minimal [http.ResponseWriter] + [http.Flusher]
// that discards all writes.
type discardResponseWriter struct {
	header http.Header
}

func (w *discardResponseWriter) Header() http.Header         { return w.header }
func (w *discardResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *discardResponseWriter) WriteHeader(int)             {}
func (w *discardResponseWriter) Flush()                      {}

// failingResponseWriter is a minimal [http.ResponseWriter] + [http.Flusher]
// whose Write always fails, simulating a broken downstream connection.
type failingResponseWriter struct {
	header http.Header
}

func (w *failingResponseWriter) Header() http.Header       { return w.header }
func (w *failingResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (w *failingResponseWriter) WriteHeader(int)           {}
func (w *failingResponseWriter) Flush()                    {}

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
				w := &discardResponseWriter{header: http.Header{}}
				w.header.Set("Content-Type", "application/json")
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

// BenchmarkResponseRecorderStreamChunked mirrors BenchmarkResponseRecorderStream
// but issues one Write per SSE event.
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
				w := &discardResponseWriter{header: http.Header{}}
				w.header.Set("Content-Type", "text/event-stream")
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
