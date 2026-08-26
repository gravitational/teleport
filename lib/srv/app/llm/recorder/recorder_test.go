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

package recorder_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/gravitational/teleport/lib/srv/app/llm/recorder"
	llmtesting "github.com/gravitational/teleport/lib/srv/app/llm/testing"
)

// TestNonStreaming exercises the provider-agnostic non-streaming
// (application/json) recorder pipeline against a stub endpoint.
func TestNonStreaming(t *testing.T) {
	t.Parallel()

	const successResponse = `{"In":15,"Out":20}`

	for name, tc := range map[string]struct {
		providerStatusCode           int
		providerBody                 string
		providerContentType          string
		expectedDownstreamBody       require.ValueAssertionFunc
		expectedDownstreamStatusCode require.ValueAssertionFunc
		expectedContentLength        require.ValueAssertionFunc
		expectedRecorder             require.ValueAssertionFunc
	}{
		"success forwards body and records usage": {
			providerStatusCode:  http.StatusOK,
			providerBody:        successResponse,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, successResponse, i1.(string), i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(len(successResponse)),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.Equal(tt, len(successResponse), rec.Written())
				require.Equal(tt, 15, rec.InputTokensCount(), i2...)
				require.Equal(tt, 20, rec.OutputTokensCount(), i2...)
				require.NoError(tt, rec.Err(), i2...)
			},
		},
		"unsupported content-type forces error": {
			providerStatusCode:  http.StatusOK,
			providerBody:        "Non-JSON result",
			providerContentType: "text/html",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body := i1.(string)
				require.Contains(tt, body, llmerrors.ErrBadResponse.Error(), "expected Teleport message")
				require.Contains(tt, body, "text/html", "expected unsupported content-type detail")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusInternalServerError),
			expectedContentLength:        require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"error status is parsed by endpoint": {
			providerStatusCode:  http.StatusUnauthorized,
			providerBody:        `{"error":"boom"}`,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body := i1.(string)
				require.Contains(tt, body, "stub provider error", "expected endpoint-provided detail")
				require.Contains(tt, body, llmerrors.ErrUnauthorized.Error(), "expected mapped Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusUnauthorized),
			expectedContentLength:        require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.Equal(tt, 0, rec.OutputTokensCount(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrUnauthorized, i2...)
			},
		},
		"empty success body is bad response": {
			providerStatusCode:           http.StatusOK,
			providerBody:                 "",
			providerContentType:          "application/json",
			expectedDownstreamBody:       require.Empty,
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(0),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.Empty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"empty error body falls back to bad response": {
			providerStatusCode:  http.StatusUnauthorized,
			providerBody:        "",
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Contains(tt, i1.(string), llmerrors.ErrBadResponse.Error(), "expected fallback Teleport message")
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusUnauthorized),
			expectedContentLength:        require.Empty,
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"unparseable success body forwarded as-is": {
			providerStatusCode:  http.StatusOK,
			providerBody:        `{"In":`,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, `{"In":`, i1.(string), i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(len(`{"In":`)),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.Equal(tt, len(`{"In":`), rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
		"no write header success uses default 200": {
			// WriteHeader is not called.
			providerStatusCode:  0,
			providerBody:        successResponse,
			providerContentType: "application/json",
			expectedDownstreamBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, successResponse, i1.(string), i2...)
			},
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(len(successResponse)),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
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
				body := i1.(string)
				require.Contains(tt, body, llmerrors.ErrBadResponse.Error(), "expected Teleport message")
				require.Contains(tt, body, "text/html", "expected unsupported content-type detail")
			},
			// Without WriteHeader the recorder cannot force a 500 status nor
			// strip the original Content-Length, so the status stays 200.
			expectedDownstreamStatusCode: expectStatus(http.StatusOK),
			expectedContentLength:        expectContentLength(len("Non-JSON result")),
			expectedRecorder: func(tt require.TestingT, i1 any, i2 ...any) {
				rec := i1.(*recorder.ResponseRecorder)
				require.NotEmpty(tt, rec.Written(), i2...)
				require.ErrorIs(tt, rec.Err(), llmerrors.ErrBadResponse, i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			run := func(t *testing.T, write func(rec *recorder.ResponseRecorder)) {
				w := httptest.NewRecorder()
				rec, err := recorder.NewResponseRecorder(slog.Default(), w, newFakeEndpoint())
				require.NoError(t, err)

				rec.Header().Add("Content-Type", tc.providerContentType)
				rec.Header().Add("Content-Length", strconv.Itoa(len(tc.providerBody)))
				// A status code of 0 simulates a caller that never calls
				// WriteHeader, exercising the default 200 status path.
				if tc.providerStatusCode != 0 {
					rec.WriteHeader(tc.providerStatusCode)
				}
				write(rec)

				require.NoError(t, rec.Close())

				resp := w.Result()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				tc.expectedRecorder(t, rec)
				tc.expectedDownstreamStatusCode(t, resp.StatusCode)
				tc.expectedContentLength(t, w.Header().Get("Content-Length"))
				tc.expectedDownstreamBody(t, string(body))
			}

			// Provider responses can arrive all at once (single write) or
			// across multiple writes (more realistic for large responses).
			t.Run("single write", func(t *testing.T) {
				run(t, func(rec *recorder.ResponseRecorder) {
					if len(tc.providerBody) > 0 {
						_, err := io.WriteString(rec, tc.providerBody)
						require.NoError(t, err)
					}
				})
			})
			t.Run("multi write", func(t *testing.T) {
				run(t, func(rec *recorder.ResponseRecorder) {
					for chunk := range slices.Chunk([]byte(tc.providerBody), 1) {
						_, err := rec.Write(chunk)
						require.NoError(t, err)
					}
				})
			})
		})
	}
}

// TestStreaming exercises the provider-agnostic streaming (text/event-stream)
// recorder pipeline: the recorder hands the stream to the endpoint's ProcessSSE
// and records the bytes written, usage and error it reports.
func TestStreaming(t *testing.T) {
	t.Parallel()

	const stream = "event: message\n" + `data: {"hello":"world"}` + "\n\n"

	t.Run("success records usage and forwards stream", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rec, err := recorder.NewResponseRecorder(slog.Default(), w, newFakeEndpoint())
		require.NoError(t, err)

		rec.Header().Add("Content-Type", "text/event-stream")
		rec.WriteHeader(http.StatusOK)
		_, err = io.WriteString(rec, stream)
		require.NoError(t, err)
		require.NoError(t, rec.Close())

		resp := w.Result()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, stream, string(body))
		require.Equal(t, len(stream), rec.Written())
		require.Equal(t, 15, rec.InputTokensCount())
		require.Equal(t, 60, rec.OutputTokensCount())
		require.NoError(t, rec.Err())
	})

	t.Run("processor error surfaces via Err", func(t *testing.T) {
		t.Parallel()

		ep := newFakeEndpoint()
		ep.processSSE = func(_ context.Context, _ *slog.Logger, r io.ReadCloser, _ io.Writer) (int, int, error) {
			defer r.Close()
			_, _ = io.ReadAll(r)
			return 0, 0, llmerrors.ErrRejected
		}

		w := httptest.NewRecorder()
		rec, err := recorder.NewResponseRecorder(slog.Default(), w, ep)
		require.NoError(t, err)

		rec.Header().Add("Content-Type", "text/event-stream")
		rec.WriteHeader(http.StatusOK)
		_, err = io.WriteString(rec, stream)
		require.NoError(t, err)
		require.NoError(t, rec.Close())

		require.ErrorIs(t, rec.Err(), llmerrors.ErrRejected)
	})

	t.Run("empty stream writes nothing", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rec, err := recorder.NewResponseRecorder(slog.Default(), w, newFakeEndpoint())
		require.NoError(t, err)

		rec.Header().Add("Content-Type", "text/event-stream")
		rec.WriteHeader(http.StatusOK)
		// No body written, so the SSE processing goroutine never starts.
		require.NoError(t, rec.Close())

		require.Empty(t, rec.Written())
	})
}

// TestDownstreamWriteFailure ensures the recorder still surfaces the semantic
// error via Err() when writing to a broken downstream connection.
func TestDownstreamWriteFailure(t *testing.T) {
	t.Parallel()

	t.Run("non-streaming error write", func(t *testing.T) {
		t.Parallel()

		w := llmtesting.NewFailingResponseWriter("application/json")
		rec, err := recorder.NewResponseRecorder(slog.Default(), w, newFakeEndpoint())
		require.NoError(t, err)

		rec.WriteHeader(http.StatusUnauthorized)
		// Empty error body drives the ParseError fallback.
		require.NoError(t, rec.Close())
		require.ErrorIs(t, rec.Err(), llmerrors.ErrBadResponse)
	})

	t.Run("streaming processor error", func(t *testing.T) {
		t.Parallel()

		ep := newFakeEndpoint()
		ep.processSSE = func(_ context.Context, _ *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
			defer r.Close()
			_, _ = io.ReadAll(r)
			// The provider error must be preserved even when the downstream
			// write fails.
			_, _ = w.Write([]byte("ignored"))
			return 0, 0, llmerrors.ErrRejected
		}

		w := llmtesting.NewFailingResponseWriter("text/event-stream")
		rec, err := recorder.NewResponseRecorder(slog.Default(), w, ep)
		require.NoError(t, err)

		rec.WriteHeader(http.StatusOK)
		_, err = io.WriteString(rec, "event: message\ndata: {}\n\n")
		require.NoError(t, err)
		// Close waits on the streaming goroutine, so Err() is populated.
		require.NoError(t, rec.Close())
		require.ErrorIs(t, rec.Err(), llmerrors.ErrRejected)
	})
}

// fakeEndpoint is a configurable [recorder.Endpoint] stub used to exercise the
// provider-agnostic recorder pipeline in isolation.
type fakeEndpoint struct {
	parseError   func(int, []byte) (*llmerrors.ProviderError, error)
	marshalError func(error) []byte
	parseUsage   func([]byte) (int, int, error)
	processSSE   func(context.Context, *slog.Logger, io.ReadCloser, io.Writer) (int, int, error)
}

func (e fakeEndpoint) ParseError(status int, body []byte) (*llmerrors.ProviderError, error) {
	return e.parseError(status, body)
}

func (e fakeEndpoint) MarshalError(err error) []byte { return e.marshalError(err) }

func (e fakeEndpoint) ParseUsage(body []byte) (int, int, error) {
	return e.parseUsage(body)
}

func (e fakeEndpoint) ProcessSSE(ctx context.Context, log *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
	return e.processSSE(ctx, log, r, w)
}

func newFakeEndpoint() fakeEndpoint {
	return fakeEndpoint{
		parseError: func(_ int, body []byte) (*llmerrors.ProviderError, error) {
			if len(body) == 0 {
				return nil, errors.New("empty error body")
			}
			return llmerrors.NewProviderError(llmerrors.ErrUnauthorized, "stub provider error"), nil
		},
		marshalError: func(err error) []byte {
			return fmt.Appendf(nil, `{"message":%q}`, err.Error())
		},
		parseUsage: func(body []byte) (int, int, error) {
			var u struct{ In, Out int }
			if err := json.Unmarshal(body, &u); err != nil {
				return 0, 0, err
			}
			return u.In, u.Out, nil
		},
		processSSE: func(_ context.Context, _ *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
			defer r.Close()
			data, err := io.ReadAll(r)
			if err != nil {
				return 0, 0, err
			}
			if _, err := w.Write(data); err != nil {
				return 0, 0, err
			}
			return 15, 60, nil
		},
	}
}

func expectStatus(statusCode int) require.ValueAssertionFunc {
	return func(tt require.TestingT, i1 any, i2 ...any) {
		require.Equal(tt, statusCode, i1, i2...)
	}
}

func expectContentLength(length int) require.ValueAssertionFunc {
	return func(tt require.TestingT, i1 any, i2 ...any) {
		contentLength, err := strconv.Atoi(i1.(string))
		require.NoError(tt, err, "expected content length to not be empty")
		require.Equal(tt, length, contentLength, i2...)
	}
}
