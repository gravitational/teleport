// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wait

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

const defaultMaxTries = 7

type untilConfig struct {
	retryConfig *retryutils.LinearConfig
	maxTries    int
}

type UntilOpts func(*untilConfig)

// WithRetryConfig sets a non-default retry configuration. Default is a linear retry with 100ms steps.
func WithRetryConfig(retryConfig *retryutils.LinearConfig) UntilOpts {
	return func(opts *untilConfig) {
		opts.retryConfig = retryConfig
	}
}

// WithMaxTries sets a non-default maximum number of tries. Default is 7.
func WithMaxTries(maxTries int) UntilOpts {
	return func(opts *untilConfig) {
		opts.maxTries = maxTries
	}
}

// Until runs the [get] function until the [check] function returns nil.
// [check] returning an error will cause the function to retry later.
// Early error exit can be performed by returning [retryutils.PermanentRetryError]
// (e.g. in case of unexpected error).
func Until[T any](
	ctx context.Context,
	get func(context.Context) (T, error),
	check func(T, error) error,
	opts ...UntilOpts,
) (T, error) {
	var resp T

	// Apply options
	config := new(untilConfig)
	for _, opt := range opts {
		opt(config)
	}

	// Set defaults
	if config.retryConfig == nil {
		config.retryConfig = &retryutils.LinearConfig{
			First:  0,
			Step:   100 * time.Millisecond,
			Max:    time.Second,
			Jitter: retryutils.DefaultJitter,
		}
	}
	if config.maxTries == 0 {
		config.maxTries = defaultMaxTries
	}

	backoff, err := retryutils.NewLinear(*config.retryConfig)
	if err != nil {
		return resp, trace.Wrap(err, "building backoff, this is a bug")
	}

	var tries int

	err = backoff.For(ctx, func() error {
		tries += 1
		resp, err = get(ctx)
		checkErr := check(resp, err)
		if checkErr != nil && tries >= config.maxTries {
			// Too many tries, and the last one failed, stop now.
			return retryutils.PermanentRetryError(checkErr)
		}
		return checkErr
	})
	return resp, trace.Wrap(err)
}

// UntilFound runs a command until it stops returning a [trace.NotFoundErr].
// This allows clients to wait for a resource to be created and in cache.
// This is required to avoid races for new cached resources not falling back to
// backend in case of cache miss.
//
// IMPORTANT: this function only guarantees that the Auth Service instance the client
// is currently connected has the resource in its cache. This provides no guarantees
// about the other Auth Service instances.
func UntilFound[T any](
	ctx context.Context,
	get func(context.Context) (T, error),
	opts ...UntilOpts,
) (T, error) {
	return Until(ctx, get, checkFound, opts...)
}

func checkFound[T any](res T, err error) error {
	if err == nil {
		// No error, resource found.
		return nil
	}
	if trace.IsNotFound(err) {
		// Resource not found, retry later.
		return trace.CompareFailed("resource not found: %s", err.Error())
	}
	// Unexpected error, stop now.
	return retryutils.PermanentRetryError(err)
}

// UntilNotFound runs a command until it returns a [trace.NotFoundErr].
// This allows clients to wait for a resource to be deleted and removed from cache.
// This is required to avoid races for newly deleted resources.
//
// IMPORTANT: this function only guarantees that the Auth Service instance the client
// is currently connected doesn't have the resource in its cache. This provides no guarantees
// about the other Auth Service instances.
func UntilNotFound[T any](
	ctx context.Context,
	get func(context.Context) (T, error),
	opts ...UntilOpts,
) (T, error) {
	return Until(ctx, get, checkNotFound, opts...)
}

func checkNotFound[T any](res T, err error) error {
	if err == nil {
		// No error, condition resource is still found.
		return trace.CompareFailed("resource still exists")
	}
	if trace.IsNotFound(err) {
		// Resource was not found, condition met.
		return nil
	}
	// Unexpected error, stop now.
	return retryutils.PermanentRetryError(err)
}

type withMetadata interface {
	GetMetadata() *headerv1.Metadata
}

// UntilRevisionChange gets a resource until its revision changes.
// This allows clients to wait for a change on an already cached resource to be propagated.
// This is required to avoid races for newly updated resources.
//
// IMPORTANT: this function only guarantees that the Auth Service instance the client
// is currently connected has the resource in its cache. This provides no guarantees
// about the other Auth Service instances.
//
// While one might want to wait for a specific revision, as often done in unit tests,
// this is not possible without a risk of deadlock. Teleport revisions are not monotonically
// increasing, and when two backend writes happen quickly, the client has no way to know
// if it observes an earlier or later revision.
func UntilRevisionChange[T withMetadata](
	ctx context.Context,
	initialRevision string,
	get func(context.Context) (T, error),
	opts ...UntilOpts,
) (T, error) {
	return Until(ctx, get, checkRevisionChange[T](initialRevision), opts...)
}

func checkRevisionChange[T withMetadata](initialRevision string) func(res T, err error) error {
	return func(res T, err error) error {
		if err != nil {
			// Unexpected error, stop now.
			return retryutils.PermanentRetryError(err)
		}
		if metadata := res.GetMetadata(); metadata.Revision == initialRevision {
			// Resource revision did not change, retry later.
			return trace.CompareFailed("Resource %q still has old revision %q", metadata.Name, initialRevision)
		}
		// Resource has a new revision, condition met.
		return nil
	}
}
