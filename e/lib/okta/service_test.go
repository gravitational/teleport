package okta

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestBecomeLeader(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	ap := newTestAccessPoint(t, clock)

	const numServices = 10
	services := make([]*Service, numServices)

	// Start all of the services.
	ctx := context.Background()
	for i := 0; i < numServices; i++ {
		services[i], _, _ = newTestService(t, ap)
		services[i].hostID = strconv.Itoa(i)
		services[i].clock = clock
		require.NoError(t, services[i].Start(ctx))
	}

	orgURL := services[0].orgURLBase64

	var holderID string
	// Each service should grab the semaphore as the other services are stopped.
	for serviceCount := 0; serviceCount < numServices-1; serviceCount++ {
		// One semaphore lease should be retrieved.
		var semaphores []types.Semaphore
		require.Eventually(t, func() bool {
			var err error
			semaphores, err = ap.GetSemaphores(ctx, types.SemaphoreFilter{
				SemaphoreKind: semaphoreKind,
				SemaphoreName: orgURL,
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
			leaderIndex = slices.IndexFunc(services, func(s *Service) bool {
				return s.IsLeader()
			})
			return leaderIndex != -1 && strconv.Itoa(leaderIndex) == holderID
		}, 10*time.Second, 250*time.Millisecond, "leader host ID (%d) does not match holder ID (%s)", leaderIndex, holderID)

		require.NoError(t, services[leaderIndex].Shutdown())
		require.NoError(t, services[leaderIndex].Close(ctx))

		// Advance so that another service grabs the semaphore.
		clock.Advance(semaphoreRenewal * 100)
	}
}
