// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package delay

import (
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestFixed(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	clock := clockwork.NewFakeClock()
	start := clock.Now()

	const first = time.Second
	const ivl = time.Minute
	const offset = 10 * time.Second

	ticker := New(Params{
		FirstInterval: first,
		FixedInterval: ivl,

		clock: clock,
	})
	defer ticker.Stop()

	clock.BlockUntil(1)
	// advance by more than the first interval but not enough to reach the
	// second tick
	clock.Advance(offset)

	tick := <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first), tick)

	clock.BlockUntil(1)
	clock.Advance(ivl)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first+ivl), tick)

	// difference in behavior compared to a [time.Ticker], but it's only really
	// noticeable after lagging behind for at least two ticks and only when
	// jitters are not involved

	clock.BlockUntil(1)
	clock.Advance(3 * ivl)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	// this is the expected tick since the previous one that was received
	require.Equal(start.Add(first+2*ivl), tick)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	// this is the tick (received immediately) corresponding to a whole interval
	// since the wall time at which the ticker was advanced the last time
	require.Equal(start.Add(offset+4*ivl), tick)
}

func TestJitter(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	clock := clockwork.NewFakeClock()
	start := clock.Now()

	const first = time.Second
	const ivl = time.Minute

	// we test a variable jitter that alternates between the full interval and
	// half of it, deterministically
	var applyJitter bool
	ticker := New(Params{
		FirstInterval: first,
		FixedInterval: ivl,

		Jitter: func(d time.Duration) time.Duration {
			if applyJitter {
				d /= 2
			}
			applyJitter = !applyJitter
			return d
		},

		clock: clock,
	})
	defer ticker.Stop()

	clock.BlockUntil(1)
	clock.Advance(first)

	tick := <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first), tick)

	clock.BlockUntil(1)
	clock.Advance(ivl)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first+ivl), tick)

	clock.BlockUntil(1)
	clock.Advance(ivl / 2)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first+ivl+ivl/2), tick)

	clock.BlockUntil(1)
	clock.Advance(ivl)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first+ivl+ivl/2+ivl), tick)
}

func TestVariable(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	clock := clockwork.NewFakeClock()
	start := clock.Now()

	const first = time.Second
	ivl := interval.NewVariableDuration(interval.VariableDurationConfig{
		MinDuration: 2 * time.Minute,
		MaxDuration: 4 * time.Minute,
		Step:        1,
	})

	// deterministic jitter, always half the actual time
	ticker := New(Params{
		FirstInterval:    first,
		VariableInterval: ivl,
		Jitter: func(d time.Duration) time.Duration {
			return d / 2
		},

		clock: clock,
	})
	defer ticker.Stop()

	clock.BlockUntil(1)
	clock.Advance(first)

	tick := <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first), tick)

	clock.BlockUntil(1)
	clock.Advance(time.Minute)

	// this is enough to saturate the VariableDuration, so we are going to hit
	// the max duration every time
	ivl.Add(100)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first+time.Minute), tick)

	clock.BlockUntil(1)
	clock.Advance(2 * time.Minute)

	tick = <-ticker.Elapsed()
	ticker.Advance(tick)
	require.Equal(start.Add(first+3*time.Minute), tick)
}
