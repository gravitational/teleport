/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package azureresourcegraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// testSubscriptionID is a syntactically valid subscription UUID used across tests.
const testSubscriptionID = "060a97ea-3a57-4218-9be5-dba3f19ff2b5"

func testSubscriptionIDs(count int) []string {
	subscriptions := make([]string, count)
	for i := range subscriptions {
		subscriptions[i] = fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1)
	}
	return subscriptions
}

// fakeCredential is a stub azcore.TokenCredential.
// It returns the configured token (defaulting to "test-token"), or err if set, and records every call so tests can assert how it was used.
type fakeCredential struct {
	token  string
	err    error
	calls  int
	scopes [][]string
}

func (c *fakeCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.calls++
	c.scopes = append(c.scopes, slices.Clone(opts.Scopes))
	if c.err != nil {
		return azcore.AccessToken{}, c.err
	}

	token := c.token
	if token == "" {
		token = "test-token"
	}
	return azcore.AccessToken{
		Token:     token,
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonHTTPResponse(status int, header http.Header, body string) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// httpClientReturning is an *http.Client that returns the same response for every request.
func httpClientReturning(status int, header http.Header, body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonHTTPResponse(status, header, body), nil
	})}
}

// retryAfterResponse builds a 429 response carrying a Retry-After header for the given duration.
func retryAfterResponse(duration time.Duration) *http.Response {
	return jsonHTTPResponse(
		http.StatusTooManyRequests,
		http.Header{"Retry-After": []string{strconv.FormatInt(int64(duration/time.Second), 10)}},
		resourceGraphErrorBody("TooManyRequests", "slow down"),
	)
}

func resourceGraphErrorBody(code, message string) string {
	return `{"error":{"code":"` + code + `","message":"` + message + `","details":[{"code":"DetailCode","message":"Detail message"}]}}`
}

func newTestRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequest(
		http.MethodPost,
		"https://management.azure.com/providers/Microsoft.ResourceGraph/resources?api-version=2022-10-01",
		strings.NewReader(`{}`),
	)
	require.NoError(t, err)
	return req
}

func clientWithCustomHTTPTransport(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()

	client, err := NewClient(
		&fakeCredential{},
		[]string{testSubscriptionID},
		WithLogger(logtest.NewLogger()),
		WithHTTPClient(&http.Client{Transport: transport}),
	)
	require.NoError(t, err)
	return client
}

// newClientWithBodies returns a client whose transport replies with the given bodies (as 200 OK responses) in order,
// recording each decoded request into requests when it is non-nil.
// It fails the test on an unexpected extra request.
func newClientWithBodies(t *testing.T, bodies []string, requests *[]resourcesAPIRequest) *Client {
	t.Helper()

	nextBody := 0
	return clientWithCustomHTTPTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if requests != nil {
			*requests = append(*requests, decodeResourceGraphRequest(t, req))
		}

		if nextBody >= len(bodies) {
			require.Failf(t, "unexpected request", "unexpected request %d (only %d bodies queued)", nextBody+1, len(bodies))
		}
		body := bodies[nextBody]
		nextBody++
		return jsonHTTPResponse(http.StatusOK, nil, body), nil
	}))
}

func decodeResourceGraphRequest(t *testing.T, req *http.Request) resourcesAPIRequest {
	t.Helper()

	var request resourcesAPIRequest
	require.NoError(t, json.NewDecoder(req.Body).Decode(&request))
	return request
}

func collectSeq[T any](seq iter.Seq2[T, error]) ([]T, []error) {
	var items []T
	var errs []error
	for item, err := range seq {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		items = append(items, item)
	}
	return items, errs
}

// testRow is a minimal row type used to exercise generic decoding.
type testRow struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// rowQuery is a TypedQuery[testRow] whose query string (or construction error)
// is configurable, used to drive Iterator/CollectAll without a real query.
type rowQuery struct {
	query string
	err   error
}

func (q rowQuery) Query() (string, error) {
	if q.err != nil {
		return "", q.err
	}
	return q.query, nil
}

func (q rowQuery) Item() testRow {
	return testRow{}
}

// pageBody builds a resource-graph page response body.
func pageBody(skipToken, truncated string, rows ...string) string {
	return fmt.Sprintf(
		`{"$skipToken":%q,"count":%d,"totalRecords":%d,"resultTruncated":%q,"data":[%s]}`,
		skipToken, len(rows), len(rows), truncated, strings.Join(rows, ","),
	)
}

func TestNewClient(t *testing.T) {
	t.Run("rejects invalid input", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			tokenProvider azcore.TokenCredential
			subscriptions []string
		}{
			{name: "missing token provider", subscriptions: []string{testSubscriptionID}},
			{name: "missing subscriptions", tokenProvider: &fakeCredential{}},
			{name: "invalid subscription ID", tokenProvider: &fakeCredential{}, subscriptions: []string{"not-a-uuid"}},
			{name: "one invalid subscription among valid ones", tokenProvider: &fakeCredential{}, subscriptions: []string{testSubscriptionID, "bogus"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				client, err := NewClient(tc.tokenProvider, tc.subscriptions)
				require.Truef(t, trace.IsBadParameter(err), "expected bad parameter error, got %T: %v", err, err)
				require.Nil(t, client)
			})
		}
	})

	t.Run("invalid subscription is named in the error", func(t *testing.T) {
		_, err := NewClient(&fakeCredential{}, []string{"not-a-uuid"})
		require.True(t, trace.IsBadParameter(err))
		require.Contains(t, err.Error(), "not-a-uuid")
	})

	t.Run("applies defaults", func(t *testing.T) {
		client, err := NewClient(&fakeCredential{}, []string{testSubscriptionID})
		require.NoError(t, err)
		require.NotNil(t, client.logger)
		require.Same(t, http.DefaultClient, client.httpClient)
	})

	t.Run("applies options", func(t *testing.T) {
		logger := logtest.NewLogger()
		httpClient := &http.Client{}
		client, err := NewClient(&fakeCredential{}, []string{testSubscriptionID},
			WithLogger(logger),
			WithHTTPClient(httpClient),
		)
		require.NoError(t, err)
		require.Same(t, logger, client.logger)
		require.Same(t, httpClient, client.httpClient)
	})

	t.Run("clones the subscriptions slice", func(t *testing.T) {
		subscriptions := []string{testSubscriptionID}
		client, err := NewClient(&fakeCredential{}, subscriptions)
		require.NoError(t, err)
		subscriptions[0] = "11111111-1111-1111-1111-111111111111"
		require.Equal(t, testSubscriptionID, client.subscriptions[0])
	})
}

func TestQueryOptionDefaults(t *testing.T) {
	def := defaultQueryOptions()
	require.False(t, def.skipMalformedRows)
	require.True(t, def.retryOnRateLimitErrors)

	opts := defaultQueryOptions()
	SkipMalformedRows()(&opts)
	require.True(t, opts.skipMalformedRows)

	opts = defaultQueryOptions()
	WithoutRetryOnRateLimitErrors()(&opts)
	require.False(t, opts.retryOnRateLimitErrors)
}

func TestResourcesAPIError_Error(t *testing.T) {
	e := &resourcesAPIError{
		Code:    "BadRequest",
		Message: "the query is invalid",
		Details: []resourcesAPIErrorDetail{
			{Code: "InvalidQuery", Message: "syntax error"},
			{Code: "Hint", Message: "check the where clause"},
		},
	}
	want := "(BadRequest) the query is invalid. Details: [InvalidQuery: syntax error][Hint: check the where clause]"
	require.Equal(t, want, e.Error())

	noDetails := &resourcesAPIError{Code: "X", Message: "y"}
	require.Equal(t, "(X) y. Details: ", noDetails.Error())
}

func TestLogAndWaitRetryAfter_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := logAndWaitRetryAfter(ctx, logtest.NewLogger(), &rateLimitError{Err: trace.LimitExceeded("rl")})
	require.ErrorIs(t, err, context.Canceled)
}

func TestFetchPageClassifiesAzureResponses(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		page, err := fetchPage(httpClientReturning(http.StatusOK, nil, `{
			"$skipToken": "next-page",
			"count": 1,
			"totalRecords": 2,
				"resultTruncated": "false",
				"data": [{"id": "vm-1"}]
			}`), newTestRequest(t))
		require.NoError(t, err)

		require.Equal(t, "next-page", page.SkipToken)
		require.Equal(t, 1, page.Count)
		require.Equal(t, 2, page.TotalRecords)
		var row map[string]string
		require.NoError(t, json.Unmarshal(page.Data[0], &row))
		require.Equal(t, "vm-1", row["id"])
	})

	t.Run("error statuses", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			status      int
			predicate   func(error) bool
			wantSnippet string
		}{
			{name: "unauthorized", status: http.StatusUnauthorized, predicate: trace.IsAccessDenied, wantSnippet: "access denied"},
			{name: "forbidden", status: http.StatusForbidden, predicate: trace.IsAccessDenied, wantSnippet: "access denied"},
			{name: "bad request", status: http.StatusBadRequest, predicate: trace.IsBadParameter, wantSnippet: "bad request"},
			{name: "rate limited", status: http.StatusTooManyRequests, predicate: trace.IsLimitExceeded, wantSnippet: "too many requests"},
			{name: "unexpected status", status: http.StatusTeapot, predicate: trace.IsBadParameter, wantSnippet: "418"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				page, err := fetchPage(
					httpClientReturning(
						tc.status,
						http.Header{"Retry-After": []string{"11"}},
						resourceGraphErrorBody("APIError", "request failed"),
					),
					newTestRequest(t),
				)
				require.Error(t, err)
				require.Nil(t, page)
				require.Truef(t, tc.predicate(err), "unexpected error %T: %v", err, err)
				require.Contains(t, err.Error(), tc.wantSnippet)
			})
		}
	})

	t.Run("server error keeps the status code", func(t *testing.T) {
		_, err := fetchPage(
			httpClientReturning(http.StatusServiceUnavailable, nil, "upstream exploded"),
			newTestRequest(t),
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "503")
	})

	t.Run("unmarshalable body", func(t *testing.T) {
		_, err := fetchPage(
			httpClientReturning(http.StatusOK, nil, "this is not json"),
			newTestRequest(t),
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal response body")
	})

	t.Run("transport error", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		})}
		_, err := fetchPage(client, newTestRequest(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to execute resource graph query")
	})

	t.Run("rate limit keeps retry-after", func(t *testing.T) {
		_, err := fetchPage(
			httpClientReturning(
				http.StatusTooManyRequests,
				http.Header{"Retry-After": []string{"11"}},
				resourceGraphErrorBody("TooManyRequests", "slow down"),
			),
			newTestRequest(t),
		)
		require.Error(t, err)

		var rateLimitErr *rateLimitError
		require.ErrorAs(t, err, &rateLimitErr)
		require.Equal(t, 11*time.Second, rateLimitErr.RetryAfter)
	})

	t.Run("rate limit with non-valid body", func(t *testing.T) {
		_, err := fetchPage(
			httpClientReturning(
				http.StatusTooManyRequests,
				http.Header{"Retry-After": []string{"11"}},
				"this is not json",
			),
			newTestRequest(t),
		)
		require.Error(t, err)

		var rateLimitErr *rateLimitError
		require.ErrorAs(t, err, &rateLimitErr)
		require.Equal(t, 11*time.Second, rateLimitErr.RetryAfter)
	})
}

func TestClientFetchPageBuildsAzureRequest(t *testing.T) {
	var captured resourcesAPIRequest
	credential := &fakeCredential{token: "secret-token"}

	client, err := NewClient(
		credential,
		[]string{testSubscriptionID},
		WithLogger(logtest.NewLogger()),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "https://management.azure.com/providers/Microsoft.ResourceGraph/resources?api-version=2022-10-01", req.URL.String())
			require.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))
			require.NoError(t, json.NewDecoder(req.Body).Decode(&captured))
			return jsonHTTPResponse(http.StatusOK, nil, `{"data": []}`), nil
		})}),
	)
	require.NoError(t, err)

	_, err = client.fetchPage(t.Context(), "resources | limit 1", "skip-me", []string{testSubscriptionID})
	require.NoError(t, err)

	require.Equal(t, 1, credential.calls)
	require.Equal(t, []string{"https://management.azure.com/.default"}, credential.scopes[0])
	require.Equal(t, "resources | limit 1", captured.Query)
	require.Equal(t, []string{testSubscriptionID}, captured.Subscriptions)
	require.Equal(t, "skip-me", captured.Options.SkipToken)
	require.NotNil(t, captured.Options.Top)
	require.Equal(t, resourceGraphPageSize, *captured.Options.Top)
	require.Equal(t, "objectArray", captured.Options.ResultFormat)
}

func TestClientFetchPageReturnsCredentialError(t *testing.T) {
	transportWasCalled := false
	client, err := NewClient(
		&fakeCredential{err: errors.New("token failed")},
		[]string{testSubscriptionID},
		WithLogger(logtest.NewLogger()),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			transportWasCalled = true
			return jsonHTTPResponse(http.StatusOK, nil, `{"data":[]}`), nil
		})}),
	)
	require.NoError(t, err)

	page, err := client.fetchPage(t.Context(), "resources | limit 1", "", []string{testSubscriptionID})
	require.Error(t, err)
	require.Nil(t, page)
	require.Contains(t, err.Error(), "failed to get token")
	require.False(t, transportWasCalled)
}

func TestCollectAllAndIterator(t *testing.T) {
	t.Run("collects typed rows across pages", func(t *testing.T) {
		var requests []resourcesAPIRequest
		client := newClientWithBodies(t, []string{
			`{"$skipToken":"next","data":[{"id":"vm-1","count":1}]}`,
			`{"data":[{"id":"vm-2","count":2}]}`,
		}, &requests)

		rows, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
		require.NoError(t, err)

		require.Equal(t, []testRow{{ID: "vm-1", Count: 1}, {ID: "vm-2", Count: 2}}, rows)
		// The skip token from page 1 must be propagated to page 2.
		require.Len(t, requests, 2)
		require.Empty(t, requests[0].Options.SkipToken)
		require.Equal(t, "next", requests[1].Options.SkipToken)
	})

	t.Run("chunks subscriptions into batches of 1000", func(t *testing.T) {
		subscriptions := testSubscriptionIDs(1001)
		credential := &fakeCredential{}
		var requests []resourcesAPIRequest
		nextBody := 0

		client, err := NewClient(
			credential,
			subscriptions,
			WithLogger(logtest.NewLogger()),
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, decodeResourceGraphRequest(t, req))

				bodies := []string{
					`{"data":[{"id":"chunk-1","count":1000}]}`,
					`{"data":[{"id":"chunk-2","count":1}]}`,
				}
				if nextBody >= len(bodies) {
					require.Failf(t, "unexpected request", "unexpected request %d (only %d bodies queued)", nextBody+1, len(bodies))
				}
				body := bodies[nextBody]
				nextBody++
				return jsonHTTPResponse(http.StatusOK, nil, body), nil
			})}),
		)
		require.NoError(t, err)

		rows, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
		require.NoError(t, err)
		require.Equal(t, []testRow{{ID: "chunk-1", Count: 1000}, {ID: "chunk-2", Count: 1}}, rows)

		require.Equal(t, 2, credential.calls)
		require.Len(t, requests, 2)
		require.Equal(t, subscriptions[:1000], requests[0].Subscriptions)
		require.Equal(t, subscriptions[1000:], requests[1].Subscriptions)
	})

	t.Run("rejects nil inputs and query construction errors", func(t *testing.T) {
		_, errs := collectSeq(Iterator(t.Context(), nil, rowQuery{query: "test query"}))
		require.Len(t, errs, 1)
		require.True(t, trace.IsBadParameter(errs[0]))

		client := newClientWithBodies(t, []string{`{"data":[]}`}, nil)
		var typedQuery TypedQuery[testRow]
		_, errs = collectSeq(Iterator(t.Context(), client, typedQuery))
		require.Len(t, errs, 1)
		require.True(t, trace.IsBadParameter(errs[0]))

		queryErr := errors.New("build query failed")
		_, errs = collectSeq(Iterator(t.Context(), client, rowQuery{err: queryErr}))
		require.Len(t, errs, 1)
		require.ErrorIs(t, errs[0], queryErr)
	})

	t.Run("handles malformed rows", func(t *testing.T) {
		client := newClientWithBodies(t, []string{`{"data":[{"id":1},{"id":"vm-2","count":2}]}`}, nil)

		rows, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal row")
		require.Nil(t, rows)

		client = newClientWithBodies(t, []string{`{"data":[{"id":1},{"id":"vm-2","count":2}]}`}, nil)
		rows, err = CollectAll(t.Context(), client, rowQuery{query: "test query"}, SkipMalformedRows())
		require.NoError(t, err)
		require.Equal(t, []testRow{{ID: "vm-2", Count: 2}}, rows)
	})

	t.Run("returns truncation error after yielding current page", func(t *testing.T) {
		client := newClientWithBodies(t, []string{`{"resultTruncated":"true","data":[{"id":"vm-1"}]}`}, nil)

		rows, errs := collectSeq(Iterator(t.Context(), client, rowQuery{query: "test query"}))
		require.Equal(t, []testRow{{ID: "vm-1"}}, rows)
		require.Len(t, errs, 1)
		require.True(t, trace.IsBadParameter(errs[0]))
		require.Contains(t, errs[0].Error(), "truncated")
	})
}

func TestIteratorStopsWhenConsumerBreaks(t *testing.T) {
	var calls int
	client := clientWithCustomHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonHTTPResponse(http.StatusOK, nil, pageBody("", "false", `{"id":"vm-1"}`, `{"id":"vm-2"}`)), nil
	}))

	var seen int
	for row, err := range Iterator(t.Context(), client, rowQuery{query: "test query"}) {
		require.NoError(t, err)
		_ = row
		seen++
		break // consumer stops early
	}
	require.Equal(t, 1, seen)
	require.Equal(t, 1, calls)
}

func TestCollectAll_MaxPages(t *testing.T) {
	// Always return a non-empty skip token with no rows so pagination never
	// terminates on its own; the max-pages safeguard must stop it.
	var calls int
	client := clientWithCustomHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonHTTPResponse(http.StatusOK, nil, pageBody("keep-going", "false")), nil
	}))

	_, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "maximum page limit")
	require.Equal(t, resourceGraphMaxPages, calls)
}

func TestIteratorRetryBehavior(t *testing.T) {
	t.Run("retries the same page after Retry-After", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var callTimes []time.Time
			var skipTokens []string

			client := clientWithCustomHTTPTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				request := decodeResourceGraphRequest(t, req)
				skipTokens = append(skipTokens, request.Options.SkipToken)
				callTimes = append(callTimes, time.Now())

				if len(callTimes) == 1 {
					return jsonHTTPResponse(
						http.StatusTooManyRequests,
						http.Header{"Retry-After": []string{"6"}},
						resourceGraphErrorBody("TooManyRequests", "slow down"),
					), nil
				}
				return jsonHTTPResponse(http.StatusOK, nil, `{"data":[{"id":"vm-1","count":1}]}`), nil
			}))

			rows, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
			require.NoError(t, err)
			require.Equal(t, []testRow{{ID: "vm-1", Count: 1}}, rows)
			require.Len(t, callTimes, 2)
			// The rate-limited page is retried with the same (empty) skip token.
			require.Equal(t, []string{"", ""}, skipTokens)
			require.Equal(t, 6*time.Second, callTimes[1].Sub(callTimes[0]))
		})
	})

	t.Run("resets retry budget after a successful page", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			client := clientWithCustomHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++

				switch {
				case calls <= maxRetriesAfterRateLimitErrors-1:
					return retryAfterResponse(2 * time.Second), nil
				case calls == maxRetriesAfterRateLimitErrors:
					return jsonHTTPResponse(http.StatusOK, nil, `{"$skipToken":"next","data":[{"id":"vm-1"}]}`), nil
				case calls <= 2*maxRetriesAfterRateLimitErrors-1:
					return retryAfterResponse(2 * time.Second), nil
				case calls == 2*maxRetriesAfterRateLimitErrors:
					return jsonHTTPResponse(http.StatusOK, nil, `{"data":[{"id":"vm-2"}]}`), nil
				default:
					require.Failf(t, "unexpected request", "unexpected request %d", calls)
					return nil, nil
				}
			}))

			rows, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
			require.NoError(t, err)
			require.Equal(t, []testRow{{ID: "vm-1"}, {ID: "vm-2"}}, rows)
			require.Equal(t, 2*maxRetriesAfterRateLimitErrors, calls)
		})
	})

	t.Run("stops after maximum consecutive rate limits", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			calls := 0
			client := clientWithCustomHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return retryAfterResponse(2 * time.Second), nil
			}))

			rows, err := CollectAll(t.Context(), client, rowQuery{query: "test query"})
			require.Error(t, err)
			require.Nil(t, rows)
			require.Equal(t, maxRetriesAfterRateLimitErrors, calls)
			require.True(t, trace.IsLimitExceeded(err))

			// The last rate-limit response returns immediately because there is no retry budget left.
			// Only the first max-1 responses are slept on.
			wantElapsed := time.Duration(maxRetriesAfterRateLimitErrors-1) * 2 * time.Second
			require.Equal(t, wantElapsed, time.Since(start))
		})
	})

	t.Run("aborts the retry wait when the context is canceled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Every request is rate limited with no Retry-After header, so the client falls back to its 30s default wait.
			// Canceling the context must abort that wait instead of running it to completion.
			client := clientWithCustomHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonHTTPResponse(
					http.StatusTooManyRequests,
					nil,
					resourceGraphErrorBody("TooManyRequests", "slow down"),
				), nil
			}))

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			go func() {
				// Let the iterator reach its retry wait, then cancel well before the 30s default elapses.
				time.Sleep(time.Second)
				cancel()
			}()

			start := time.Now()
			_, err := CollectAll(ctx, client, rowQuery{query: "test query"})
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, time.Second, time.Since(start))
		})
	})

	t.Run("does not retry when disabled", func(t *testing.T) {
		calls := 0
		client := clientWithCustomHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return retryAfterResponse(6 * time.Second), nil
		}))

		rows, err := CollectAll(
			t.Context(),
			client,
			rowQuery{query: "test query"},
			WithoutRetryOnRateLimitErrors(),
		)
		require.Error(t, err)
		require.Nil(t, rows)
		require.Equal(t, 1, calls)
		require.True(t, trace.IsLimitExceeded(err))
	})
}
