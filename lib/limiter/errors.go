/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package limiter

import "time"

// RateLimitExceededError is returned by [RateLimiter.RegisterRequest] when the
// rate limit is exceeded. It wraps a [trace.LimitExceededError] so that
// [trace.IsLimitExceeded] returns true, and additionally exposes the Delay the
// caller should wait before the next token becomes available.
type RateLimitExceededError struct {
	// Delay is how long to wait before retrying.
	Delay time.Duration
	// cause is the underlying trace.LimitExceededError, kept for Unwrap.
	err error
}

// Error returns the error message of the underlying error.
func (e *RateLimitExceededError) Error() string { return e.err.Error() }

// Unwrap returns the underlying error, allowing errors.Is and errors.As to work.
func (e *RateLimitExceededError) Unwrap() error { return e.err }
