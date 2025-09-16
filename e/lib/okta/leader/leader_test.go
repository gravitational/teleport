package leader

import (
	"slices"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestBecomeLeader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clock := clockwork.NewRealClock()
		backend, err := memory.New(memory.Config{
			Clock: clock,
		})
		t.Cleanup(func() {
			require.NoError(t, backend.Close())
		})
		require.NoError(t, err)
		presence := local.NewPresenceService(backend)
		t.Cleanup(func() {
			require.NoError(t, presence.Close())
		})

		const numServices = 10
		var services []*Leader

		semaphoreName := uuid.NewString()
		semaphoreKind := "service"

		ctx := t.Context()
		for i := range numServices {
			item, err := New(Config{
				SemaphoreName: semaphoreName,
				SemaphoreKind: semaphoreKind,
				HostIDHolder:  strconv.Itoa(i),
				Clock:         clock,
				Semaphores:    presence,
			})
			require.NoError(t, err)
			item.Start(ctx)
			services = append(services, item)
		}

		var holderID string
		// Each service should grab the semaphore as the other services are stopped.
		for range numServices {
			synctest.Wait()
			// One semaphore lease should be retrieved.
			semaphores, err := presence.GetSemaphores(ctx, types.SemaphoreFilter{
				SemaphoreKind: semaphoreKind,
				SemaphoreName: semaphoreName,
			})
			require.NoError(t, err)

			require.Len(t, semaphores, 1)
			// Update the holder ID.
			holderID = semaphores[0].LeaseRefs()[0].Holder

			// Make sure the service denoted as the semaphore holder is the current leader, otherwise it shouldn't be.
			leaderIndex := slices.IndexFunc(services, func(s *Leader) bool {
				return s.IsLeader()
			})
			require.NotEqual(t, -1, leaderIndex, "no leader found")
			require.Equal(t, holderID, strconv.Itoa(leaderIndex), "leader host ID (%d) does not match holder ID (%s)", leaderIndex, holderID)

			require.NoError(t, services[leaderIndex].Close())

			// Wait for the service to close.
			synctest.Wait()
			time.Sleep(100 * semaphoreRenewal)
		}
	})
}
