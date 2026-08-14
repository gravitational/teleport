package dsqlbk

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
)

type testEmitter struct {
	emitted chan int
}

var _ eventEmitter = (*testEmitter)(nil)

// Emit implements [eventEmitter].
func (e *testEmitter) Emit(events ...backend.Event) (ok bool) {
	select {
	case e.emitted <- len(events):
	default:
		panic("emitted channel full")
	}
	return true
}

func (e *testEmitter) requireNotEmitted(t *testing.T) {
	require.Empty(t, e.emitted)
}

func (e *testEmitter) requireEmitted(t *testing.T, expected int) {
	require.NotEmpty(t, e.emitted)
	require.Equal(t, expected, <-e.emitted)
}

func TestEventOrderFilter(t *testing.T) {
	_, err := newEventOrderFilter(0)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	require.ErrorContains(t, err, "(this is a bug)")

	_, err = newEventOrderFilter(-1)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	require.ErrorContains(t, err, "(this is a bug)")

	f, err := newEventOrderFilter(time.Hour)
	require.NoError(t, err)

	emitter := &testEmitter{emitted: make(chan int, 1)}

	require.NoError(t, f.emitEvents(nil, emitter))
	emitter.requireNotEmitted(t)

	event := func(key string, commitTimestamp time.Duration) cdcEvent {
		return cdcEvent{
			event: &backend.Event{
				Item: backend.Item{
					Key: backend.KeyFromString(key),
				},
			},
			commitTimestamp: commitTimestamp,
		}
	}

	require.NoError(t, f.emitEvents([]cdcEvent{
		event("foo", 100),
		event("foo", 100),
	}, emitter))
	emitter.requireEmitted(t, 1)

	require.NoError(t, f.emitEvents([]cdcEvent{
		event("foo", 99),
		event("foo", 200),
	}, emitter))
	emitter.requireEmitted(t, 1)

	require.Equal(t, time.Duration(200), f.maxTimestamp)

	for i := range 2000 {
		require.NoError(t, f.emitEvents([]cdcEvent{
			event(fmt.Sprintf("bar%08d", i), 4*time.Hour+time.Duration(i)*time.Millisecond),
		}, emitter))
		emitter.requireEmitted(t, 1)
	}

	require.Equal(t, 4*time.Hour+1999*time.Millisecond, f.maxTimestamp)
	// max timestamp at the cleanup (at 1024 items) minus the grace interval
	require.Equal(t, 3*time.Hour+1022*time.Millisecond, f.timestampWatermark)
	require.Len(t, f.events, 2000)
	require.NotContains(t, f.events, "foo")

	// first cleanup at 1024 removes only foo, next cleanup at twice that
	require.Equal(t, 1023*2, f.nextCleanupSize)
}
