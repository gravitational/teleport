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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// resourceGraphPageSize is the number of results to request per page.
	// According to Azure Docs, the maximum page size is 1000, which is what we use here.
	// https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/work-with-data#paging-results
	resourceGraphPageSize int32 = 1000

	// resourceGraphMaxPages is the maximum number of pages to fetch when paginating through results.
	// Azure Docs don't specify a maximum number of pages, but setting a high limit here is a safeguard against infinite loops.
	// With a page size of 1000, fetching 10 000 pages allows us to fetch up to 10 million resources, which should be more than enough.
	resourceGraphMaxPages = 10_000

	// When Azure Resource Graph returns a 429 Too Many Requests response, the Retry-After header indicates how long to wait before retrying the request.
	// If the API returns a 429 response 10 times in a row, we stop retrying and return an error to avoid infinite loops in case of persistent rate limiting.
	maxRetriesAfterRateLimitErrors = 10

	// resultTruncatedTrue is the value of the ResultTruncated field in the API response when the results are truncated.
	resultTruncatedTrue = "true"
)

// ClientOption configures a Client during construction.
type ClientOption func(*Client)

// WithLogger sets the logger used by the client for diagnostic messages.
// If logger is nil, NewClient uses slog.Default.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithHTTPClient sets the HTTP client used for Azure Resource Graph requests.
// If httpClient is nil, NewClient uses http.DefaultClient.
func WithHTTPClient(httpClient utils.HTTPDoClient) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a Client for querying Azure Resource Graph across the provided Azure subscriptions.
//
// tokenProvider must be non-nil and concurrent-safe. It is used to acquire access tokens for authenticating requests to Azure Resource Graph.
// Each subscription must be a valid UUID.
func NewClient(tokenProvider azcore.TokenCredential, subscriptions []string, opts ...ClientOption) (*Client, error) {
	if tokenProvider == nil {
		return nil, trace.BadParameter("tokenProvider is required")
	}

	if len(subscriptions) == 0 {
		return nil, trace.BadParameter("at least one subscription is required")
	}

	for _, sub := range subscriptions {
		if _, err := uuid.Parse(sub); err != nil {
			return nil, trace.BadParameter("invalid subscription ID: %s", sub)
		}
	}

	client := &Client{
		tokenProvider: tokenProvider,
		subscriptions: slices.Clone(subscriptions),
	}
	for _, opt := range opts {
		opt(client)
	}

	if client.logger == nil {
		client.logger = slog.With(teleport.ComponentKey, "azure_resource_graph_client")
	}

	if client.httpClient == nil {
		client.httpClient = http.DefaultClient
	}

	return client, nil
}

// Client is a client for querying Azure Resource Graph.
// It is safe for concurrent use by multiple goroutines.
type Client struct {
	tokenProvider azcore.TokenCredential
	logger        *slog.Logger
	httpClient    utils.HTTPDoClient
	subscriptions []string
}

// TypedQuery describes an Azure Resource Graph query and the expected result
// type for each returned row.
type TypedQuery[T any] interface {
	// Query returns the KQL query string.
	Query() (string, error)
	// Item returns the zero value of T used for type inference and row decoding.
	Item() T
}

// QueryOption controls query behavior.
type QueryOption func(*queryOptions)

type queryOptions struct {
	skipMalformedRows      bool
	retryOnRateLimitErrors bool
}

func defaultQueryOptions() queryOptions {
	return queryOptions{
		skipMalformedRows:      false,
		retryOnRateLimitErrors: true,
	}
}

// SkipMalformedRows returns a QueryOption that skips rows which cannot be unmarshaled into the query item type.
// Without this option, the first malformed row stops iteration and is returned as an error.
func SkipMalformedRows() QueryOption {
	return func(q *queryOptions) {
		q.skipMalformedRows = true
	}
}

// WithoutRetryOnRateLimitErrors disables the default behavior of retrying queries that fail with rate limit errors.
// When this option is set, any rate limit error is returned immediately without retries.
func WithoutRetryOnRateLimitErrors() QueryOption {
	return func(q *queryOptions) {
		q.retryOnRateLimitErrors = false
	}
}

// CollectAll executes typedQuery against Azure Resource Graph and collects all results into a slice of T.
func CollectAll[T any](ctx context.Context, client *Client, typedQuery TypedQuery[T], queryOpts ...QueryOption) ([]T, error) {
	var results []T
	for item, err := range Iterator(ctx, client, typedQuery, queryOpts...) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results = append(results, item)
	}
	return results, nil
}

// Iterator executes typedQuery against Azure Resource Graph API and returns an iterator over decoded rows.
//
// The returned iterator fetches pages as it is consumed.
// Each step yields either a decoded row with a nil error, or a terminal error with the zero value of T.
// It stops when there are no more rows.
func Iterator[T any](ctx context.Context, client *Client, typedQuery TypedQuery[T], queryOpts ...QueryOption) iter.Seq2[T, error] {
	options := defaultQueryOptions()
	for _, opt := range queryOpts {
		opt(&options)
	}

	return func(yield func(T, error) bool) {
		var zero T

		if client == nil {
			yield(zero, trace.BadParameter("client is required"))
			return
		}
		if typedQuery == nil {
			yield(zero, trace.BadParameter("typedQuery is required"))
			return
		}

		logger := client.logger.With("query_type", fmt.Sprintf("%T", typedQuery))
		logger.DebugContext(ctx, "Starting query execution")

		queryString, err := typedQuery.Query()
		if err != nil {
			yield(zero, trace.Wrap(err))
			return
		}

		maxPageRetries := maxRetriesAfterRateLimitErrors
		var skipToken string

		// Azure Resource Graph API has a limit of 1000 subscriptions per query, so we chunk the subscriptions and execute the query for each chunk.
		// See https://learn.microsoft.com/en-us/azure/governance/resource-graph/troubleshoot/general#scenario-too-many-subscriptions
		for subscriptionsChunk := range slices.Chunk(client.subscriptions, 1000) {
			for currentPage := 1; currentPage <= resourceGraphMaxPages; currentPage++ {
				page, err := client.fetchPage(ctx, queryString, skipToken, subscriptionsChunk)
				if err != nil {
					maxPageRetries--

					rateLimitErr, isRateLimitErr := errors.AsType[*rateLimitError](err)
					if isRateLimitErr && options.retryOnRateLimitErrors && maxPageRetries > 0 {
						if err := logAndWaitRetryAfter(ctx, logger, rateLimitErr); err != nil {
							yield(zero, trace.Wrap(err))
							return
						}

						currentPage--
						continue
					}

					yield(zero, trace.Wrap(err))
					return
				}
				maxPageRetries = maxRetriesAfterRateLimitErrors // Reset retry counter on successful page fetch.

				skipToken = page.SkipToken

				for _, row := range page.Data {
					item := typedQuery.Item()
					if err := json.Unmarshal(row, &item); err != nil {
						if options.skipMalformedRows {
							logger.WarnContext(ctx, "Skipping malformed row", "error", err)
							continue
						}

						yield(zero, trace.Wrap(err, "failed to unmarshal row into %T", item))
						return
					}

					if !yield(item, nil) {
						return
					}
				}

				logger.DebugContext(ctx, "Processed page of results", "page", currentPage, "total_records", page.TotalRecords, "count", page.Count, "skip_token", page.SkipToken, "result_truncated", page.ResultTruncated)

				if page.ResultTruncated == resultTruncatedTrue {
					yield(zero, trace.BadParameter("query result truncated, consider creating a more specific query (eg, add location or resource group filters)"))
					return
				}

				if skipToken == "" {
					logger.DebugContext(ctx, "No more pages to fetch, ending iteration")
					break
				}

				if currentPage >= resourceGraphMaxPages {
					logger.WarnContext(ctx, "Reached maximum page limit, stopping pagination", "max_pages", resourceGraphMaxPages)
					yield(zero, trace.BadParameter("query exceeded the maximum page limit %d", resourceGraphMaxPages))
					return
				}
			}
		}
	}
}

// fetchPage retrieves a single page of the resource graph query, requesting the rows that follow skipToken (empty for the first page).
func (c *Client) fetchPage(ctx context.Context, query, skipToken string, subscriptions []string) (*resourcesAPIResponse, error) {
	resourcesBody := resourcesAPIRequest{
		Query:         query,
		Subscriptions: subscriptions,
		Options: resourcesAPIRequestOptions{
			SkipToken:    skipToken,
			Top:          new(resourceGraphPageSize),
			ResultFormat: "objectArray",
		},
	}

	resourcesBodyJSON, err := json.Marshal(resourcesBody)
	if err != nil {
		return nil, trace.Wrap(err, "failed to marshal resource graph query body")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://management.azure.com/providers/Microsoft.ResourceGraph/resources?api-version=2022-10-01",
		bytes.NewReader(resourcesBodyJSON),
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create HTTP request")
	}

	token, err := c.tokenProvider.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to get token")
	}

	request.Header.Set("Authorization", "Bearer "+token.Token)
	request.Header.Set("Content-Type", "application/json")

	page, err := fetchPage(c.httpClient, request)
	if err != nil {
		// The wrapped error still lets callers match *rateLimitError via errors.AsType.
		return nil, trace.Wrap(err, "failed to fetch resource graph page")
	}

	return page, nil
}

func logAndWaitRetryAfter(ctx context.Context, logger *slog.Logger, err *rateLimitError) error {
	// Ensure waiting time has clear boundaries.
	retryAfter := err.RetryAfter
	if retryAfter <= 0 || retryAfter > 5*time.Minute {
		retryAfter = 30 * time.Second
	}

	logger.DebugContext(ctx, "Received rate limit error, retrying after delay", "error", err.Err, "retry_after", retryAfter, "original_retry_after", err.RetryAfter)

	select {
	case <-ctx.Done():
		return trace.NewAggregate(ctx.Err(), trace.BadParameter("canceled while waiting to retry after rate limit error"))
	case <-time.After(retryAfter):
		return nil
	}
}

func fetchPage(client utils.HTTPDoClient, req *http.Request) (*resourcesAPIResponse, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "failed to execute resource graph query")
	}
	defer resp.Body.Close()

	responseBodyBS, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, trace.BadParameter("resource graph query failed with status code %d: %s", resp.StatusCode, string(responseBodyBS))
	}

	// We expect the response body to contain a JSON object either with the data or with an error.
	var pageResponse resourcesAPIResponse
	unmarshalError := json.Unmarshal(responseBodyBS, &pageResponse)

	errorDetails := pageResponse.Error.Error()
	if unmarshalError != nil {
		errorDetails = fmt.Sprintf("failed to unmarshal response body: %s", string(responseBodyBS))
	}

	statusCode := resp.StatusCode

	switch resp.StatusCode {
	case http.StatusOK:
		if unmarshalError != nil {
			return nil, trace.Wrap(unmarshalError, "failed to unmarshal response body")
		}
		return &pageResponse, nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, trace.AccessDenied("access denied (%d): %s", statusCode, errorDetails)

	case http.StatusBadRequest:
		return nil, trace.BadParameter("bad request (%d): %s", statusCode, errorDetails)

	case http.StatusTooManyRequests:
		return nil, wrapWithRetryAfterHeader(resp.Header, trace.LimitExceeded("too many requests (%d): %s", statusCode, errorDetails))

	default:
		return nil, trace.BadParameter("resource graph query failed with status code %d: %s", statusCode, errorDetails)
	}
}

type resourcesAPIResponse struct {
	SkipToken       string            `json:"$skipToken"`
	Count           int               `json:"count"`
	TotalRecords    int               `json:"totalRecords"`
	ResultTruncated string            `json:"resultTruncated"`
	Data            []json.RawMessage `json:"data"`

	Error resourcesAPIError `json:"error"`
}

type resourcesAPIError struct {
	Code    string                    `json:"code"`
	Message string                    `json:"message"`
	Details []resourcesAPIErrorDetail `json:"details"`
}

func (e *resourcesAPIError) Error() string {
	var details strings.Builder
	for _, detail := range e.Details {
		fmt.Fprintf(&details, "[%s: %s]", detail.Code, detail.Message)
	}

	return fmt.Sprintf("(%s) %s. Details: %s", e.Code, e.Message, details.String())
}

type resourcesAPIErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type resourcesAPIRequest struct {
	// The resources query.
	Query string `json:"query"`
	// Azure subscriptions against which to execute the query.
	Subscriptions []string `json:"subscriptions"`
	// The query evaluation options
	Options resourcesAPIRequestOptions `json:"options"`
}

type resourcesAPIRequestOptions struct {
	// Opaque token used for pagination. Should be set to the $skipToken value returned by the previous page to fetch the next page.
	SkipToken string `json:"$skipToken,omitempty"`
	// The maximum number of rows that the query should return.
	Top *int32 `json:"$top,omitempty"`
	// Defines the format in which query results are returned.
	ResultFormat string `json:"resultFormat,omitempty"`
}
