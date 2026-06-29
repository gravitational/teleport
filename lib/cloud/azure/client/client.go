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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// maxRetriesAfterRateLimitErrors indicates the maximum number of times to retry a request when Azure Resource Graph returns a 429 Too Many Requests response.
	// The Retry-After header indicates how long to wait before retrying the request.
	// If the API returns a 429 response 10 times in a row, we stop retrying and return an error to avoid infinite loops in case of persistent rate limiting.
	maxRetriesAfterRateLimitErrors = 10
)

// ClientOption configures a Client during construction.
type ClientOption func(*Client)

// WithHTTPClient sets the HTTP client used for Azure Resource Graph requests.
// If httpClient is nil, NewClient uses http.DefaultClient.
func WithHTTPClient(httpClient utils.HTTPDoClient) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithLogger sets the logger used for Azure Resource Graph requests.
// If logger is nil, NewClient uses slog.Default().
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithRetryOnRateLimitErrors configures the client to retry requests when rate limit errors are encountered.
func WithRetryOnRateLimitErrors() ClientOption {
	return func(c *Client) {
		c.retryOnRateLimitErrors = true
	}
}

// NewClient creates a Client for querying Azure Resource Graph across the provided Azure subscriptions.
//
// tokenProvider must be non-nil and concurrent-safe. It is used to acquire access tokens for authenticating requests to Azure Resource Graph.
// Each subscription must be a valid UUID.
func NewClient(tokenProvider azcore.TokenCredential, opts ...ClientOption) (*Client, error) {
	if tokenProvider == nil {
		return nil, trace.BadParameter("tokenProvider is required")
	}

	client := &Client{
		tokenProvider: tokenProvider,
	}
	for _, opt := range opts {
		opt(client)
	}

	if client.httpClient == nil {
		client.httpClient = http.DefaultClient
	}

	if client.logger == nil {
		client.logger = slog.Default()
	}

	return client, nil
}

// Client is a client for querying Azure Resource Graph.
// It is safe for concurrent use by multiple goroutines.
type Client struct {
	tokenProvider          azcore.TokenCredential
	httpClient             utils.HTTPDoClient
	logger                 *slog.Logger
	retryOnRateLimitErrors bool
}

// DoRequest retrieves a single page of the resource graph query, requesting the rows that follow skipToken (empty for the first page).
func DoRequest[T any](ctx context.Context, client *Client, request *http.Request) (*T, error) {
	var parsedResponse *T
	var currentRetry int
	for {
		responseBodyBS, err := singleFlightRequest(ctx, client, request)
		if err != nil {

			if rateLimitErr, isRateLimitErr := errors.AsType[*azure.RateLimitError](err); isRateLimitErr && client.retryOnRateLimitErrors {
				if currentRetry <= maxRetriesAfterRateLimitErrors {
					if err := logAndWaitRetryAfter(ctx, client.logger, rateLimitErr); err != nil {
						return nil, trace.Wrap(err)
					}
					currentRetry++
					continue
				}
				return nil, trace.NewAggregate(err, trace.BadParameter("exceeded maximum retries (%d) after rate limit errors", maxRetriesAfterRateLimitErrors))
			}

			return nil, trace.Wrap(err, "failed to fetch resource graph page")
		}

		if err := json.Unmarshal(responseBodyBS, parsedResponse); err != nil {
			return nil, trace.Wrap(err, "failed to unmarshal azure api response: %s", string(responseBodyBS))
		}
		break
	}

	return parsedResponse, nil
}

func singleFlightRequest(ctx context.Context, client *Client, request *http.Request) ([]byte, error) {
	const azureManagementScope = "https://management.azure.com/.default"

	token, err := client.tokenProvider.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{azureManagementScope},
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to get token")
	}

	request.Header.Set("Authorization", "Bearer "+token.Token)
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(request)
	if err != nil {
		return nil, trace.Wrap(err, "failed to execute resource graph query")
	}
	defer resp.Body.Close()

	responseBodyBS, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read response body (status code: %d)", resp.StatusCode)
	}
	if resp.StatusCode > http.StatusBadRequest {
		return nil, azure.ErrorFromResponse(resp)
	}

	return responseBodyBS, nil
}

func logAndWaitRetryAfter(ctx context.Context, logger *slog.Logger, err *azure.RateLimitError) error {
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
