package leader

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestBecomeLeader(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	presence := local.NewPresenceService(backend)

	const numServices = 10
	var services []*Leader

	semaphoreName := uuid.NewString()
	semaphoreKind := "service"

	ctx := context.Background()
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
	for range numServices - 1 {
		// One semaphore lease should be retrieved.
		var semaphores []types.Semaphore
		require.Eventually(t, func() bool {
			var err error
			semaphores, err = presence.GetSemaphores(ctx, types.SemaphoreFilter{
				SemaphoreKind: semaphoreKind,
				SemaphoreName: semaphoreName,
			})
			if err != nil {
				return false
			}
			// the old holder ID shouldn't be the lease holder, it should have moved to a new service
			return len(semaphores) == 1 && len(semaphores[0].LeaseRefs()) == 1 && semaphores[0].LeaseRefs()[0].Holder != holderID
		}, 5*time.Second, 250*time.Millisecond)

		// Update the holder ID.
		holderID = semaphores[0].LeaseRefs()[0].Holder

		// Make sure the service denoted as the semaphore holder is the current leader, otherwise it shouldn't be.
		var leaderIndex int
		require.Eventually(t, func() bool {
			leaderIndex = slices.IndexFunc(services, func(s *Leader) bool {
				return s.IsLeader()
			})
			return leaderIndex != -1 && strconv.Itoa(leaderIndex) == holderID
		}, 10*time.Second, 250*time.Millisecond, "leader host ID (%d) does not match holder ID (%s)", leaderIndex, holderID)

		require.NoError(t, services[leaderIndex].Close())

		// Advance so that another service grabs the semaphore.
		clock.Advance(semaphoreRenewal * 100)
	}
}
