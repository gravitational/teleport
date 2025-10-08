// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package retryutils

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	// updateWithRetryMaxRetries is the default number of retries in case of conflicts.
	updateWithRetryMaxRetries = 3
	// updateWithRetryHalfJitterBetweenAttempts is the default jitter, [UpdateWithRetry] is
	// using to wait between attempts.
	updateWithRetryHalfJitterBetweenAttempts = 2 * time.Second
)

type updateWithRetryOptions struct {
	maxRetries  int
	retryConfig RetryV2Config
}

// UpdateWithRetryOpt is the option type for [UpdateWithRetry]. See With* funcs in the package to
// see what options are available.
type UpdateWithRetryOpt func(*updateWithRetryOptions)

// WithMaxRetries changes the max retry attempts number from the default
// [updateWithRetryMaxRetries].
func WithMaxRetries(maxRetries int) UpdateWithRetryOpt {
	return func(o *updateWithRetryOptions) {
		o.maxRetries = maxRetries
	}
}

// WithRetryConfig changes the default retry configuration.
func WithRetryConfig(config RetryV2Config) UpdateWithRetryOpt {
	return func(o *updateWithRetryOptions) {
		o.retryConfig = config
	}
}

// RefreshFn refreshes the resource if needed considering if this is an retry attempt or the first
// attempt.
type RefreshFn[R any] func(ctx context.Context, isRetry bool) (R, error)

// UpdateFn conditionally updates the resource.
type UpdateFn[R any] func(ctx context.Context, resource R) error

// UpdateWithRetry tries to conditionally update a resource, by default retrying 3 times with a 2s
// half jittered constant backoff in between retries. The retry configuration can be changed with
// the option arguments.
//
// UpdateWithRetry will retry only if updateFn returns an error matched with trace.IsCompareFailed.
// It will return any other error immediately without retries.
func UpdateWithRetry[R any](ctx context.Context, clock clockwork.Clock, refreshFn RefreshFn[R], updateFn UpdateFn[R], opts ...UpdateWithRetryOpt) error {
	options := updateWithRetryOptions{
		maxRetries: updateWithRetryMaxRetries,
		retryConfig: RetryV2Config{
			Clock:  clock,
			Driver: NewConstantDriver(updateWithRetryHalfJitterBetweenAttempts),
			Max:    updateWithRetryHalfJitterBetweenAttempts,
			Jitter: HalfJitter,
		},
	}
	for _, o := range opts {
		o(&options)
	}

	retry, err := NewRetryV2(options.retryConfig)
	if err != nil {
		return trace.Wrap(err, "creating retry instance")
	}

	var updateErr error
	attempts := min(options.maxRetries+1, maxAttempts)
	for i := range attempts {
		lastAttempt := i == attempts-1

		isRetry := i > 0
		resource, err := refreshFn(ctx, isRetry)
		if err != nil {
			return trace.Wrap(err)
		}

		updateErr = updateFn(ctx, resource)
		switch {
		case !lastAttempt && trace.IsCompareFailed(updateErr):
			d := retry.Duration()
			select {
			case <-clock.After(d):
				continue
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		case updateErr != nil:
			return trace.Wrap(updateErr)
		default:
			return nil
		}
	}
	return trace.Wrap(updateErr)
}
