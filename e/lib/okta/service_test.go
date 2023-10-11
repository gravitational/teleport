/*
Copyright 2023 Gravitational, Inc.

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

package okta

import (
	"context"
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
		for i := 0; i < numServices; i++ {
			hostID := services[i].hostID
			if hostID == holderID {
				require.True(t, services[i].IsLeader(), "host id %s should be leader", hostID)

				// Close this service so that it relinquishes the semaphore.
				require.NoError(t, services[i].Shutdown())
				require.NoError(t, services[i].Close(ctx))
			} else {
				require.False(t, services[i].IsLeader(), "host id %s should not be leader", hostID)
			}

		}

		// Advance so that another service grabs the semaphore.
		clock.Advance(semaphoreRenewal * 100)
	}
}
