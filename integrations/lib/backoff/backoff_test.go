/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
