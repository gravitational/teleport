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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// maxRetriesAfterRateLimitErrors indicates the maximum number of times to retry a request when the Azure API returns a 429 Too Many Requests response.
	// The Retry-After header indicates how long to wait before retrying the request.
	// If the API returns a 429 response 10 times in a row, we stop retrying and return an error to avoid infinite loops in case of persistent rate limiting.
	maxRetriesAfterRateLimitErrors = 10
)

// ClientOption configures a Client during construction.
type ClientOption func(*Client)

// WithHTTPClient sets the HTTP client used for Azure API requests.
// If httpClient is nil, NewClient uses http.DefaultClient.
func WithHTTPClient(httpClient utils.HTTPDoClient) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithLogger sets the logger used for Azure API requests.
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

// WithClock sets the clock used to wait between retries.
// If clock is nil, NewClient uses a real clock.
func WithClock(clock clockwork.Clock) ClientOption {
	return func(c *Client) {
		c.clock = clock
	}
}

// NewClient creates a Client for making authenticated requests to the Azure Resource Manager API.
//
// tokenProvider must be non-nil and concurrent-safe. It is used to acquire access tokens for authenticating requests to the Azure Resource Manager API.
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

	if client.clock == nil {
		client.clock = clockwork.NewRealClock()
	}

	return client, nil
}

// Client is a client for making authenticated requests to the Azure Resource Manager API.
// It is safe for concurrent use by multiple goroutines.
type Client struct {
	tokenProvider          azcore.TokenCredential
	httpClient             utils.HTTPDoClient
	logger                 *slog.Logger
	retryOnRateLimitErrors bool
	clock                  clockwork.Clock
}

// DoRequest executes the given HTTP request against the Azure Resource Manager
// API and decodes the JSON response body into a value of type T. If the client
// is configured with WithRetryOnRateLimitErrors, DoRequest retries the request
// when the API responds with a rate limit error. DoRequest expects a JSON
// response body and will error if it doesn't receive one.
func DoRequest[T any](ctx context.Context, client *Client, request *http.Request) (T, error) {
	var parsedResponse T
	var currentRetry int
	for {
		// Clone the request because doSingleRequest will read and close the body
		// and we may need to retry the request if we hit a retryable error.
		attempt := request.Clone(ctx)
		if request.GetBody != nil {
			body, err := request.GetBody()
			if err != nil {
				return parsedResponse, trace.Wrap(err, "failed to clone request body")
			}
			attempt.Body = body
		}

		responseBodyBS, err := doSingleRequest(ctx, client, attempt)
		if err != nil {
			if rateLimitErr, isRateLimitErr := errors.AsType[*azure.RateLimitError](err); isRateLimitErr && client.retryOnRateLimitErrors {
				if currentRetry < maxRetriesAfterRateLimitErrors {
					if err := logAndWaitRetryAfter(ctx, client.clock, client.logger, rateLimitErr); err != nil {
						return parsedResponse, trace.Wrap(err)
					}
					currentRetry++
					continue
				}
				return parsedResponse, trace.NewAggregate(err, trace.BadParameter("exceeded maximum retries (%d) after rate limit errors", maxRetriesAfterRateLimitErrors))
			}

			return parsedResponse, trace.Wrap(err, "failed to fetch azure api response")
		}

		if err := json.Unmarshal(responseBodyBS, &parsedResponse); err != nil {
			// A failed Unmarshal may have partially populated parsedResponse,
			// so return an explicit zero value alongside the error.
			var zeroValue T
			return zeroValue, trace.Wrap(err, "failed to unmarshal azure api response: %s", string(responseBodyBS))
		}
		break
	}

	return parsedResponse, nil
}

func doSingleRequest(ctx context.Context, client *Client, request *http.Request) ([]byte, error) {
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
		return nil, trace.Wrap(err, "failed to execute azure api request")
	}
	defer resp.Body.Close()

	responseBodyBS, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read response body (status code: %d)", resp.StatusCode)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		// Rebuild resp.Body after it has been drained by utils.ReadAtMost so
		// ErrorFromResponse can read it and generate a detailed error message.
		resp.Body = io.NopCloser(bytes.NewReader(responseBodyBS))
		return nil, azure.ErrorFromResponse(resp)
	}

	return responseBodyBS, nil
}

func logAndWaitRetryAfter(ctx context.Context, clock clockwork.Clock, logger *slog.Logger, err *azure.RateLimitError) error {
	// Clamp err.RetryAfter to be a min of 0 seconds and a max of 5 minutes.
	retryAfter := max(0, min(err.RetryAfter, 5*time.Minute))

	logger.DebugContext(ctx, "Received rate limit error, retrying after delay", "error", err.Err, "retry_after", retryAfter, "original_retry_after", err.RetryAfter)

	select {
	case <-ctx.Done():
		return trace.NewAggregate(ctx.Err(), trace.BadParameter("canceled while waiting to retry after rate limit error"))
	case <-clock.After(retryAfter):
		return nil
	}
}
