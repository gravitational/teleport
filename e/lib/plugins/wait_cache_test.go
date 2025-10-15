package plugins

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_waitForCacheReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		events := &fakeEvents{}
		eventsLate := &fakeEvents{}

		results := make(chan struct {
			name string
		})

		// We fire two cache waiters
		go func() {
			err := waitForCacheReady(t.Context(), eventsLate, "test-late")
			assert.NoError(t, err)
			results <- struct{ name string }{name: "test-late"}
		}()
		go func() {
			err := waitForCacheReady(t.Context(), events, "test")
			assert.NoError(t, err)
			results <- struct{ name string }{name: "test"}
		}()

		// We wait for the cache waiters to wait
		synctest.Wait()

		// We check that none returned yet
		select {
		case result := <-results:
			require.Fail(t, "should not have received result yet: %s", result)
		default:
		}

		// We let the first waiter finish and validate that it returned
		events.send(types.Event{
			Type: types.OpInit,
		})
		result := <-results
		require.Equal(t, "test", result.name)

		// We make sure the other waiter is still waiting
		synctest.Wait()
		select {
		case result := <-results:
			require.Fail(t, "should not have received result yet: %s", result)
		default:
		}

		// We unblock the second waiter
		eventsLate.send(types.Event{
			Type: types.OpInit,
		})
		result = <-results
		require.Equal(t, "test-late", result.name)
	})

}
