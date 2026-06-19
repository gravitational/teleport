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
	"net/http"
	"strconv"
	"strings"
	"time"
)

// rateLimitError is returned by Azure Resource Graph client methods when the server signals a rate or concurrency limit.
// It wraps a [trace.LimitExceededError] and carries the retry-after duration extracted from the API response.
type rateLimitError struct {
	// RetryAfter is the value of the "retry-after" header, or 0 if the header was absent.
	RetryAfter time.Duration
	// Err is the underlying LimitExceeded trace error.
	Err error
}

// Error returns the underlying error message.
func (e *rateLimitError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error, allowing errors.Is and errors.As to work with rateLimitError.
func (e *rateLimitError) Unwrap() error {
	return e.Err
}

// wrapWithRetryAfterHeader converts an API error into a [*rateLimitError] when the
// error is a limit-exceeded error, extracting the retry-after value from the
// supplied header.
func wrapWithRetryAfterHeader(header http.Header, apiErr error) error {
	return &rateLimitError{
		Err:        apiErr,
		RetryAfter: extractRetryAfterDuration(header),
	}
}

// extractRetryAfterDuration extracts the retry-after duration as documented by Azure.
// When it is not present or cannot be parsed, it returns 0.
func extractRetryAfterDuration(header http.Header) time.Duration {
	// Retry-After header returns the number of seconds to wait before retrying.
	retryAfterSeconds, err := strconv.Atoi(header.Get("Retry-After"))
	if err == nil {
		return time.Duration(retryAfterSeconds) * time.Second
	}

	// This comes in the following format: X-Ms-User-Quota-Resets-After: 00:00:05
	timeParts := strings.Split(header.Get("X-Ms-User-Quota-Resets-After"), ":")
	if len(timeParts) == 3 {
		hours, err1 := strconv.Atoi(timeParts[0])
		minutes, err2 := strconv.Atoi(timeParts[1])
		seconds, err3 := strconv.Atoi(timeParts[2])
		if err1 == nil && err2 == nil && err3 == nil {
			return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
		}
	}

	return 0
}
