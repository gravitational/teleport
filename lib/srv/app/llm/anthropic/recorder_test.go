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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/httplib/sse"
	"github.com/gravitational/teleport/lib/itertools/stream"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/stretchr/testify/require"
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
				require.NoError(tt, rec.Err())
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
				require.Contains(tt, body, llmerrors.ErrUnknown.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			// In case of errors, we always expect empty Content-Length.
			expectedContentLength: require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, 146, rec.Written())
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrUnknown)
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
				require.Contains(tt, body, "invalid model", "expected error response to include original message")
				require.Contains(tt, body, llmerrors.ErrBadRequest.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusBadRequest),
			// In case of errors, we always expect empty Content-Length.
			expectedContentLength: require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, 197, rec.Written())
				require.Equal(tt, rec.OutputTokensCount(), 0, i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadRequest)
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
				require.Contains(tt, body, llmerrors.ErrUnauthorized.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusUnauthorized),
			// In case of errors, we always expect empty Content-Length.
			expectedContentLength: require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, 202, rec.Written())
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrUnauthorized)
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
				require.Empty(tt, rec.Written())
				require.NoError(tt, rec.Err(), i2...)
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
				require.Contains(tt, body, llmerrors.ErrUnknown.Error(), "expected error response to include Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusUnauthorized),
			expectedContentLength:        require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec, _ := i1.(*ResponseRecorder)
				require.Equal(tt, 146, rec.Written())
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrUnknown)
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
				require.Equal(tt, 42, rec.Written())
				require.NoError(tt, rec.Err(), i2...)
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
				rec, err := NewResponseRecorder(t.Context(), slog.Default(), w)
				require.NoError(t, err)

				rec.Header().Add("Content-Type", tc.providerContentType)
				rec.Header().Add("Content-Length", strconv.Itoa(len(tc.providerBody)))
				rec.WriteHeader(tc.providerStatusCode)
				if len(tc.providerBody) > 0 {
					_, err = io.WriteString(rec, tc.providerBody)
					require.NoError(t, err)
				}

				rec.Close()

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
				rec, err := NewResponseRecorder(t.Context(), slog.Default(), w)
				require.NoError(t, err)

				rec.Header().Add("Content-Type", tc.providerContentType)
				rec.Header().Add("Content-Length", strconv.Itoa(len(tc.providerBody)))
				rec.WriteHeader(tc.providerStatusCode)
				for chunk := range slices.Chunk([]byte(tc.providerBody), 1) {
					_, err = rec.Write(chunk)
					require.NoError(t, err)
				}

				rec.Close()

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
			rec, err := NewResponseRecorder(t.Context(), slog.Default(), w)
			require.NoError(t, err)
			tc.providerResp(rec)
			rec.Close()

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			tc.expectedRecorder(t, rec)
			tc.expectedDownstreamStatusCode(t, resp.StatusCode)
			tc.expectedDownstreamBody(t, string(body))
		})
	}
}

func readSSEOneEvent(t require.TestingT, str string) sse.Event {
	events, err := stream.Collect(sse.ReadEvents(strings.NewReader(str)))
	require.NoError(t, err)
	require.Len(t, events, 1)
	return events[0]
}
