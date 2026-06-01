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

package scim

import (
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestWrapRateLimitErr(t *testing.T) {
	t.Run("nil error passthrough", func(t *testing.T) {
		got := wrapRateLimitErr(metadata.MD{}, nil)
		require.NoError(t, got)
	})

	t.Run("non-limit error passthrough", func(t *testing.T) {
		err := trace.NotFound("not found")
		got := wrapRateLimitErr(metadata.MD{}, err)
		require.ErrorIs(t, got, err)
		var rlErr *RateLimitError
		require.False(t, errors.As(got, &rlErr))
	})

	t.Run("limit exceeded without retry-after trailer", func(t *testing.T) {
		err := trace.LimitExceeded("too many requests")
		got := wrapRateLimitErr(metadata.MD{}, err)
		var rlErr *RateLimitError
		require.ErrorAs(t, got, &rlErr)
		require.Equal(t, int64(0), rlErr.RetryAfterSeconds)
		require.True(t, trace.IsLimitExceeded(got))
	})

	t.Run("limit exceeded with retry-after trailer", func(t *testing.T) {
		err := trace.LimitExceeded("too many requests")
		trailer := metadata.MD{"retry-after": []string{"42"}}
		got := wrapRateLimitErr(trailer, err)
		var rlErr *RateLimitError
		require.ErrorAs(t, got, &rlErr)
		require.Equal(t, int64(42), rlErr.RetryAfterSeconds)
		require.True(t, trace.IsLimitExceeded(got))
	})

	t.Run("limit exceeded with unparseable retry-after trailer", func(t *testing.T) {
		err := trace.LimitExceeded("too many requests")
		trailer := metadata.MD{"retry-after": []string{"not-a-number"}}
		got := wrapRateLimitErr(trailer, err)
		var rlErr *RateLimitError
		require.ErrorAs(t, got, &rlErr)
		require.Equal(t, int64(0), rlErr.RetryAfterSeconds)
	})
}
