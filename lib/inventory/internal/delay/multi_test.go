package delay

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestMultiBasics(t *testing.T) {
	t.Parallel()

	multi := NewMulti[int](MultiParams{
		FixedInterval: time.Millisecond * 20,
	})

	// verify that delay is in an initial state that will never fire
	require.Nil(t, multi.target)
	require.Nil(t, multi.Elapsed())

	for i := 1; i <= 10; i++ {
		// add a subinterval
		multi.Add(i)
	}

	require.Equal(t, 1, multi.Current())

	for i := 0; i < 30; i++ {
		<-multi.Elapsed()
		require.Equal(t, i%10+1, multi.Current())
		multi.Advance(time.Now())
	}

	// remove some subintervals
	for i := 1; i <= 8; i++ {
		multi.Remove(i)
	}

	// verify that remaining subintervals are still being serviced
	for i := 0; i < 30; i++ {
		k := 10
		if i%2 == 0 {
			k = 9
		}
		<-multi.Elapsed()
		require.Equal(t, k, multi.Current())
		multi.Advance(time.Now())
	}

	multi.Remove(9)
	multi.Remove(10)

	// verify that the delay is in a state that will never fire
	require.Nil(t, multi.target)
	require.Nil(t, multi.Elapsed())
}

func TestMultiJitter(t *testing.T) {
	t.Parallel()

	var jitterCalled atomic.Bool
	fakeJitter := func(d time.Duration) time.Duration {
		jitterCalled.Store(true)
		return time.Millisecond * 20
	}

	multi := NewMulti[int](MultiParams{
		FixedInterval: time.Hour,
		Jitter:        fakeJitter,
	})

	for i := 0; i < 10; i++ {
		multi.Add(i + 1)
	}

	for i := 0; i < 10; i++ {
		select {
		case now := <-multi.Elapsed():
			multi.Advance(now)
		case <-time.After(time.Second * 10):
			t.Fatal("timeout waiting for delay to fire")
		}
		require.True(t, jitterCalled.Swap(false))
	}
}

func TestMultiVariable(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	start := clock.Now()

	const first = time.Second
	ivl := interval.NewVariableDuration(interval.VariableDurationConfig{
		MinDuration: 2 * time.Minute,
		MaxDuration: 4 * time.Minute,
		Step:        1,
	})

	// deterministic jitter, always half the actual time
	multi := NewMulti[int](MultiParams{
		FirstInterval:    first,
		VariableInterval: ivl,
		Jitter: func(d time.Duration) time.Duration {
			return d / 2
		},

		clock: clock,
	})
	defer multi.Stop()

	multi.Add(1)

	clock.BlockUntil(1)
	clock.Advance(first)

	tick := <-multi.Elapsed()
	multi.Advance(tick)
	require.Equal(t, start.Add(first), tick)

	clock.BlockUntil(1)
	clock.Advance(time.Minute)

	// this is enough to saturate the VariableDuration, so we are going to hit
	// the max duration every time
	ivl.Add(100)

	tick = <-multi.Elapsed()
	multi.Advance(tick)
	require.Equal(t, start.Add(first+time.Minute), tick)

	clock.BlockUntil(1)
	clock.Advance(2 * time.Minute)

	tick = <-multi.Elapsed()
	multi.Advance(tick)
	require.Equal(t, start.Add(first+3*time.Minute), tick)
}
