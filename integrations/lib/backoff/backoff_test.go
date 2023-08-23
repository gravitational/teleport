/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backoff

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestDecorr(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClockAt(time.Unix(0, 0))
	base := 20 * time.Millisecond
	cap := 2 * time.Second
	backoff := NewDecorr(base, cap, clock)

	// Check exponential bounds.
	for max := 3 * base; max < cap; max = 3 * max {
		dur, err := measure(ctx, clock, func() error { return backoff.Do(ctx) })
		require.NoError(t, err)
		require.Greater(t, dur, base)
		require.LessOrEqual(t, dur, max)
	}

	// Check that exponential growth threshold.
	for i := 0; i < 2; i++ {
		dur, err := measure(ctx, clock, func() error { return backoff.Do(ctx) })
		require.NoError(t, err)
		require.Greater(t, dur, base)
		require.LessOrEqual(t, dur, cap)
	}
}
