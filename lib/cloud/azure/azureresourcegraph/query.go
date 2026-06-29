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
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/cloud/azure/client"
)

const (
	// resourceGraphMaxPages is the maximum number of pages to fetch when paginating through results.
	// Azure Docs don't specify a maximum number of pages, but setting a high limit here is a safeguard against infinite loops.
	// With a page size of 1000, fetching 10 000 pages allows us to fetch up to 10 million resources, which should be more than enough.
	resourceGraphMaxPages = 10_000

	// resultTruncatedTrue is the value of the ResultTruncated field in the API response when the results are truncated.
	resultTruncatedTrue = "true"

	// resourceGraphPageSize is the number of results to request per page.
	// According to Azure Docs, the maximum page size is 1000, which is what we use here.
	// https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/work-with-data#paging-results
	resourceGraphPageSize int32 = 1000
)

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
func CollectAll[T any](ctx context.Context, logger *slog.Logger, subscriptions []string, client *client.Client, tokenProvider azcore.TokenCredential, typedQuery TypedQuery[T], queryOpts ...QueryOption) ([]T, error) {
	var results []T
	for item, err := range Iterator(ctx, logger, subscriptions, client, tokenProvider, typedQuery, queryOpts...) {
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
func Iterator[T any](ctx context.Context, logger *slog.Logger, subscriptions []string, clt *client.Client, tokenProvider azcore.TokenCredential, typedQuery TypedQuery[T], queryOpts ...QueryOption) iter.Seq2[T, error] {
	options := defaultQueryOptions()
	for _, opt := range queryOpts {
		opt(&options)
	}

	return func(yield func(T, error) bool) {
		var zero T

		if clt == nil {
			yield(zero, trace.BadParameter("client is required"))
			return
		}
		if typedQuery == nil {
			yield(zero, trace.BadParameter("typedQuery is required"))
			return
		}
		if tokenProvider == nil {
			yield(zero, trace.BadParameter("tokenProvider is required"))
			return
		}
		if logger == nil {
			logger = slog.With(teleport.ComponentKey, "azure_resource_graph")
		}

		logger := logger.With("query_type", fmt.Sprintf("%T", typedQuery))
		logger.DebugContext(ctx, "Starting query execution")

		queryString, err := typedQuery.Query()
		if err != nil {
			yield(zero, trace.Wrap(err))
			return
		}

		var skipToken string

		// Azure Resource Graph API has a limit of 1000 subscriptions per query, so we chunk the subscriptions and execute the query for each chunk.
		// See https://learn.microsoft.com/en-us/azure/governance/resource-graph/troubleshoot/general#scenario-too-many-subscriptions
		for subscriptionsChunk := range slices.Chunk(subscriptions, 1000) {
			for currentPage := 1; currentPage <= resourceGraphMaxPages; currentPage++ {
				resourcesBody := resourcesAPIRequest{
					Query:         queryString,
					Subscriptions: subscriptionsChunk,
					Options: resourcesAPIRequestOptions{
						SkipToken:    skipToken,
						Top:          new(resourceGraphPageSize),
						ResultFormat: "objectArray",
					},
				}
				resourcesBodyJSON, err := json.Marshal(resourcesBody)
				if err != nil {
					yield(zero, trace.Wrap(err, "failed to marshal resource graph query body"))
					return
				}
				httpRequest, err := http.NewRequest(http.MethodPost, "/providers/Microsoft.ResourceGraph/resources", bytes.NewReader(resourcesBodyJSON))
				if err != nil {
					yield(zero, trace.Wrap(err))
					return
				}

				page, err := client.DoRequest[resourcesAPIResponse](ctx, clt, httpRequest)
				if err != nil {
					yield(zero, trace.Wrap(err))
					return
				}
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

type resourcesAPIResponse struct {
	SkipToken       string            `json:"$skipToken"`
	Count           int               `json:"count"`
	TotalRecords    int               `json:"totalRecords"`
	ResultTruncated string            `json:"resultTruncated"`
	Data            []json.RawMessage `json:"data"`
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
