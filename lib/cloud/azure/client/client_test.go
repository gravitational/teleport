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

package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/azure"
)

// testPayload is the type DoRequest decodes JSON responses into.
type testPayload struct {
	Value string `json:"value"`
}

// fakeTokenCredential is an azcore.TokenCredential that returns a static token
// or an error and records how many times it was invoked.
type fakeTokenCredential struct {
	token string
	err   error
	calls atomic.Int32
}

func (f *fakeTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.calls.Add(1)
	if f.err != nil {
		return azcore.AccessToken{}, f.err
	}
	return azcore.AccessToken{Token: f.token}, nil
}

// recordedRequest captures the parts of an outgoing request the tests assert on.
type recordedRequest struct {
	method        string
	url           string
	authorization string
	contentType   string
	body          string
}

// fakeHTTPClient is a utils.HTTPDoClient that returns responses based on the
// callers provided respond function. It also records the requests it receives.
type fakeHTTPClient struct {
	respond func(attempt int, req *http.Request) (*http.Response, error)

	mu       sync.Mutex
	requests []recordedRequest
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	rec := recordedRequest{
		method:        req.Method,
		url:           req.URL.String(),
		authorization: req.Header.Get("Authorization"),
		contentType:   req.Header.Get("Content-Type"),
	}
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			f.mu.Unlock()
			return nil, err
		}
		rec.body = string(body)
	}
	attempt := len(f.requests)
	f.requests = append(f.requests, rec)
	f.mu.Unlock()

	return f.respond(attempt, req)
}

func (f *fakeHTTPClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeHTTPClient) recorded() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.requests...)
}

// newResponse builds an *http.Response with the given status, body and headers.
func newResponse(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     header,
	}
}

// rateLimitResponse builds a 429 Too Many Requests response. When
// retryAfterSeconds is positive, it sets the Retry-After header so the client
// waits that long before retrying.
func rateLimitResponse(retryAfterSeconds int) *http.Response {
	header := make(http.Header)
	if retryAfterSeconds > 0 {
		header.Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}
	return newResponse(http.StatusTooManyRequests, `{"error":{"code":"TooManyRequests","message":"slow down"}}`, header)
}

// newRequest builds a request against the Azure management API. When body is
// set the request is created with a rewindable body so DoRequest can
// re-send it on retries. The request's own context is unimportant: DoRequest
// re-binds the request to its own context argument via request.Clone.
func newRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://management.azure.com/subscriptions?api-version=2026-01-01", reader)
	require.NoError(t, err)
	return req
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("nil token provider is rejected", func(t *testing.T) {
		t.Parallel()
		c, err := NewClient(nil)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %T: %v", err, err)
		require.Nil(t, c)
	})

	t.Run("applies defaults", func(t *testing.T) {
		t.Parallel()
		creds := &fakeTokenCredential{}
		c, err := NewClient(creds)
		require.NoError(t, err)
		require.Same(t, creds, c.tokenProvider)
		require.Same(t, http.DefaultClient, c.httpClient)
		require.NotNil(t, c.logger)
		require.NotNil(t, c.clock)
		require.False(t, c.retryOnRateLimitErrors)
	})

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()
		httpClient := &fakeHTTPClient{}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		clock := clockwork.NewFakeClock()

		c, err := NewClient(&fakeTokenCredential{},
			WithHTTPClient(httpClient),
			WithLogger(logger),
			WithClock(clock),
			WithRetryOnRateLimitErrors(),
		)
		require.NoError(t, err)
		require.Same(t, httpClient, c.httpClient)
		require.Same(t, logger, c.logger)
		require.Same(t, clock, c.clock)
		require.True(t, c.retryOnRateLimitErrors)
	})
}

func TestDoRequest_Success(t *testing.T) {
	t.Parallel()

	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, `{"value":"hello"}`, nil), nil
		},
	}
	creds := &fakeTokenCredential{token: "abc123"}
	c, err := NewClient(creds, WithHTTPClient(httpClient))
	require.NoError(t, err)

	got, err := DoRequest[testPayload](t.Context(), c, newRequest(t, `{"req":true}`))
	require.NoError(t, err)
	require.Equal(t, testPayload{Value: "hello"}, got)

	require.Equal(t, 1, httpClient.callCount())
	require.EqualValues(t, 1, creds.calls.Load())

	reqs := httpClient.recorded()
	require.Len(t, reqs, 1)
	require.Equal(t, "Bearer abc123", reqs[0].authorization, "Authorization header must carry the token")
	require.Equal(t, "application/json", reqs[0].contentType)
	require.Equal(t, `{"req":true}`, reqs[0].body, "request body must be forwarded")
}

func TestDoRequest_TokenError(t *testing.T) {
	t.Parallel()

	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			t.Error("HTTP client must not be called when token acquisition fails")
			return nil, errors.New("unexpected call")
		},
	}
	c, err := NewClient(&fakeTokenCredential{err: errors.New("error")}, WithHTTPClient(httpClient))
	require.NoError(t, err)

	_, err = DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to get token")
	require.Zero(t, httpClient.callCount())
}

func TestDoRequest_HTTPClientError(t *testing.T) {
	t.Parallel()

	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		},
	}
	c, err := NewClient(&fakeTokenCredential{token: "token"}, WithHTTPClient(httpClient))
	require.NoError(t, err)

	_, err = DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to execute azure api request")
	require.ErrorContains(t, err, "connection refused")
}

func TestDoRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, `invalid json`, nil), nil
		},
	}
	c, err := NewClient(&fakeTokenCredential{token: "token"}, WithHTTPClient(httpClient))
	require.NoError(t, err)

	got, err := DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to unmarshal azure api response")
	// A failed unmarshal must return an explicit zero value.
	require.Equal(t, testPayload{}, got)
}

// TestDoRequest_ErrorStatusMapping verifies that error status codes are run
// through azure.ErrorFromResponse and surfaced as the corresponding trace
// error type. Only status-based mappings are exercised here: doSingleRequest
// drains the response body before conversion, so body-dependent sub-codes are
// out of scope for this layer.
func TestDoRequest_ErrorStatusMapping(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		status int
		isType func(error) bool
	}{
		{name: "403 maps to AccessDenied", status: http.StatusForbidden, isType: trace.IsAccessDenied},
		{name: "404 maps to NotFound", status: http.StatusNotFound, isType: trace.IsNotFound},
		{name: "409 maps to AlreadyExists", status: http.StatusConflict, isType: trace.IsAlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			httpClient := &fakeHTTPClient{
				respond: func(int, *http.Request) (*http.Response, error) {
					return newResponse(tc.status, "", nil), nil
				},
			}
			c, err := NewClient(&fakeTokenCredential{token: "token"}, WithHTTPClient(httpClient))
			require.NoError(t, err)

			_, err = DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
			require.Error(t, err)
			require.ErrorContains(t, err, "failed to fetch azure api response")
			require.True(t, tc.isType(err), "unexpected error type %T: %v", err, err)
			require.Equal(t, 1, httpClient.callCount(), "error responses must not be retried by default")
		})
	}
}

func TestDoRequest_RateLimitNotRetriedByDefault(t *testing.T) {
	t.Parallel()

	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return rateLimitResponse(5), nil
		},
	}

	c, err := NewClient(&fakeTokenCredential{token: "token"}, WithHTTPClient(httpClient))
	require.NoError(t, err)

	_, err = DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to fetch azure api response")

	var rateLimitErr *azure.RateLimitError
	require.ErrorAs(t, err, &rateLimitErr)
	require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded, got %T: %v", err, err)

	require.Equal(t, 1, httpClient.callCount(), "must not retry when retry-on-rate-limit is disabled")
}

func TestDoRequest_NonRateLimitErrorNotRetried(t *testing.T) {
	t.Parallel()

	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return newResponse(http.StatusNotFound, "", nil), nil
		},
	}

	c, err := NewClient(&fakeTokenCredential{token: "token"},
		WithHTTPClient(httpClient),
		WithClock(clockwork.NewFakeClock()),
		WithRetryOnRateLimitErrors(),
	)
	require.NoError(t, err)

	_, err = DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %T: %v", err, err)
	require.Equal(t, 1, httpClient.callCount())
}

func TestDoRequest_RateLimitRetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	httpClient := &fakeHTTPClient{
		respond: func(attempt int, _ *http.Request) (*http.Response, error) {
			if attempt == 0 {
				return rateLimitResponse(5), nil
			}
			return newResponse(http.StatusOK, `{"value":"ok"}`, nil), nil
		},
	}
	c, err := NewClient(&fakeTokenCredential{token: "token"},
		WithHTTPClient(httpClient),
		WithClock(clock),
		WithRetryOnRateLimitErrors(),
	)
	require.NoError(t, err)

	type result struct {
		payload testPayload
		err     error
	}
	resCh := make(chan result, 1)
	go func() {
		payload, err := DoRequest[testPayload](t.Context(), c, newRequest(t, `{"n":1}`))
		resCh <- result{payload, err}
	}()

	// First attempt hit the rate limit; the client is now blocked waiting to retry.
	require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
	require.Equal(t, 1, httpClient.callCount())

	// Advancing past the Retry-After delay releases the retry, which succeeds.
	clock.Advance(5 * time.Second)

	select {
	case res := <-resCh:
		require.NoError(t, res.err)
		require.Equal(t, testPayload{Value: "ok"}, res.payload)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for DoRequest to return")
	}

	require.Equal(t, 2, httpClient.callCount())
	reqs := httpClient.recorded()
	require.Len(t, reqs, 2)
	// The body and Authorization header must be re-sent on the retry.
	for i, req := range reqs {
		require.Equal(t, `{"n":1}`, req.body, "attempt %d must re-send the request body", i)
		require.Equal(t, "Bearer token", req.authorization, "attempt %d must re-send the auth header", i)
	}
}

func TestDoRequest_RateLimitExceedsMaxRetries(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return rateLimitResponse(5), nil
		},
	}
	c, err := NewClient(&fakeTokenCredential{token: "token"},
		WithHTTPClient(httpClient),
		WithClock(clock),
		WithRetryOnRateLimitErrors(),
	)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		_, err := DoRequest[testPayload](t.Context(), c, newRequest(t, ""))
		errCh <- err
	}()

	// Make DoRequest retry until we hit maxRetriesAfterRateLimitErrors
	for range maxRetriesAfterRateLimitErrors {
		require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
		clock.Advance(5 * time.Second)
	}

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.ErrorContains(t, err, "exceeded maximum retries")
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for DoRequest to give up")
	}

	require.Equal(t, maxRetriesAfterRateLimitErrors+1, httpClient.callCount(),
		"expected one initial attempt plus one attempt per retry")
}

func TestDoRequest_RateLimitContextCanceledDuringWait(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	httpClient := &fakeHTTPClient{
		respond: func(int, *http.Request) (*http.Response, error) {
			return rateLimitResponse(0), nil
		},
	}
	c, err := NewClient(&fakeTokenCredential{token: "token"},
		WithHTTPClient(httpClient),
		WithClock(clock),
		WithRetryOnRateLimitErrors(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		_, err := DoRequest[testPayload](ctx, c, newRequest(t, ""))
		errCh <- err
	}()

	// Wait until the client is blocked in the retry delay, then cancel.
	require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.ErrorContains(t, err, "canceled while waiting to retry after rate limit error")
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for DoRequest to observe cancellation")
	}

	require.Equal(t, 1, httpClient.callCount(), "no further attempts after cancellation")
}
