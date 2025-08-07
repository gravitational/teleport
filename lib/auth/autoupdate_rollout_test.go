/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"fmt"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestSampleAgentsFromGroup(t *testing.T) {
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	auth := &Server{
		cancelFunc: func() {},
		clock:      clock,
		ServerID:   uuid.NewString(),
		logger:     logtest.NewLogger(),
		Services: &Services{
			// The inventory is running heartbeats on the background.
			// If we don't create a presence service this will cause panics.
			PresenceInternal: local.NewPresenceService(bk),
		},
	}
	controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
	auth.inventory = controller
	t.Cleanup(func() {
		auth.Close()
	})

	const (
		testNodeCount         = 1000
		testGroupName         = "my-group"
		testCatchAllGroupName = "catch-all-group"
	)

	testGroups := []string{testGroupName, testCatchAllGroupName}

	for range testNodeCount {
		updaterID := uuid.New()
		stream := newFakeControlStream()
		controller.RegisterControlStream(stream, &proto.UpstreamInventoryHello{
			Services:         types.SystemRoles{types.RoleNode}.StringSlice(),
			Version:          "1.2.3",
			ServerID:         uuid.NewString(),
			ExternalUpgrader: types.UpgraderKindTeleportUpdate,
			UpdaterInfo: &types.UpdaterV2Info{
				UpdateUUID:    updaterID[:],
				UpdaterStatus: types.UpdaterStatus_UPDATER_STATUS_OK,
				UpdateGroup:   testGroupName,
			},
		})
		t.Cleanup(stream.close)
	}

	// Nodes that just registered are ignored, we advance the clock so our fixtures are not filtered out by filterHandler().
	clock.Advance(2 * time.Minute)

	// Text execution: check that we sample the correct amount of canaries
	const sampleSize = 10
	canaries, err := auth.SampleAgentsFromAutoUpdateGroup(t.Context(), testGroupName, sampleSize, testGroups)
	require.NoError(t, err)
	require.Len(t, canaries, sampleSize)

	// Test execution: check that there were no duplicates in the samples
	canarySet := make(map[string]*autoupdatev1pb.Canary)
	for _, canary := range canaries {
		canarySet[canary.UpdaterId] = canary
	}
	require.Len(t, canarySet, sampleSize, "some canary got duplicated")

	canaries2, err := auth.SampleAgentsFromAutoUpdateGroup(t.Context(), testGroupName, sampleSize, testGroups)
	require.NoError(t, err)
	require.Len(t, canaries2, sampleSize)
	canarySet = make(map[string]*autoupdatev1pb.Canary)
	for _, canary := range canaries2 {
		canarySet[canary.UpdaterId] = canary
	}
	require.Len(t, canarySet, sampleSize, "some canary got duplicated")

	// Text execution: check that the random looks sane
	var conflicts int
	for i := range sampleSize {
		if canaries[i].UpdaterId == canaries2[i].UpdaterId {
			conflicts++
		}
	}
	// The probability of having 4 nodes sampled at the same position twice in
	// a row is 2e-10.
	require.Less(t, conflicts, 4)

	// Test execution: check that agents not belonging to any group are sampled whe requesting the catch-all group.
	canariesCatchAll, err := auth.SampleAgentsFromAutoUpdateGroup(t.Context(), testCatchAllGroupName, sampleSize, []string{"group-a", testCatchAllGroupName})
	require.NoError(t, err)
	require.Len(t, canariesCatchAll, sampleSize)
	canarySet = make(map[string]*autoupdatev1pb.Canary)
	for _, canary := range canariesCatchAll {
		canarySet[canary.UpdaterId] = canary
	}
	require.Len(t, canarySet, sampleSize, "some canary got duplicated")

	// Test execution: check that agents belonging to the catch-all group are sampled.
	canariesCatchAll, err = auth.SampleAgentsFromAutoUpdateGroup(t.Context(), testGroupName, sampleSize, []string{"group-a", testGroupName})
	require.NoError(t, err)
	require.Len(t, canariesCatchAll, sampleSize)
	canarySet = make(map[string]*autoupdatev1pb.Canary)
	for _, canary := range canariesCatchAll {
		canarySet[canary.UpdaterId] = canary
	}
	require.Len(t, canarySet, sampleSize, "some canary got duplicated")
}

func TestLookupAgentInInventory(t *testing.T) {
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	auth := &Server{
		cancelFunc: func() {},
		clock:      clock,
		ServerID:   uuid.NewString(),
		logger:     logtest.NewLogger(),
		Services: &Services{
			// The inventory is running heartbeats on the background.
			// If we don't create a presence service this will cause panics.
			PresenceInternal: local.NewPresenceService(bk),
		},
	}
	controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
	auth.inventory = controller
	t.Cleanup(func() {
		auth.Close()
	})

	const testNodeCount = 5
	const testGroupName = "my-group"

	hosts := make(map[string][]*proto.UpstreamInventoryHello, testNodeCount)

	// Test setup:
	// We register X nodes, node number 1 has 1 handler, node 2 has 2, ...
	for i := range testNodeCount {
		hostID := uuid.New().String()
		updaterID := uuid.New()
		hellos := make([]*proto.UpstreamInventoryHello, i+1)
		for j := range i + 1 {
			hello := &proto.UpstreamInventoryHello{
				Services:         types.SystemRoles{types.RoleNode}.StringSlice(),
				Version:          fmt.Sprintf("1.2.%d", j),
				ServerID:         hostID,
				ExternalUpgrader: types.UpgraderKindTeleportUpdate,
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateUUID:    updaterID[:],
					UpdaterStatus: types.UpdaterStatus_UPDATER_STATUS_OK,
					UpdateGroup:   testGroupName,
				},
			}
			hellos[j] = hello

			stream := newFakeControlStream()
			controller.RegisterControlStream(stream, hello)
			t.Cleanup(stream.close)
		}
		hosts[hostID] = hellos
	}

	clock.Advance(2 * time.Minute)

	// Test execution: for each hostID registered, we get the handles and make sure we have the right length
	// and content matches.

	opts := cmp.Options{
		cmpopts.SortSlices(func(a, b *proto.UpstreamInventoryHello) bool { return a.Version > b.Version }),
		protocmp.Transform(),
	}
	for hostID, handles := range hosts {
		result, err := auth.LookupAgentInInventory(t.Context(), hostID)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(handles, result, opts))
	}
}

type fakeHandle struct {
	inventory.UpstreamHandle
	id int
}

func (f *fakeHandle) Hello() *proto.UpstreamInventoryHello {
	return &proto.UpstreamInventoryHello{
		Hostname: fmt.Sprintf("hostname-%d", f.id),
	}
}

func TestHandlerSampler(t *testing.T) {
	filterOK := func(handle inventory.UpstreamHandle) bool {
		return true
	}
	filterKO := func(handle inventory.UpstreamHandle) bool {
		return false
	}

	generateHandles := func(i int) iter.Seq[inventory.UpstreamHandle] {
		return func(yield func(inventory.UpstreamHandle) bool) {
			for j := range i {
				yield(&fakeHandle{id: j})
			}
		}
	}

	const testSampleSize = 10
	// no agents to sample
	// reservoir not filled yet
	// reservoir filled
	// filter always rejecting
	tests := []struct {
		name              string
		filter            func(handle inventory.UpstreamHandle) bool
		initialSeenCount  int
		handleFixtures    iter.Seq[inventory.UpstreamHandle]
		expectedSeenCount int
		assertSample      func(t *testing.T, reservoir []inventory.UpstreamHandle)
	}{
		{
			name:              "filter rejects all",
			filter:            filterKO,
			initialSeenCount:  0,
			handleFixtures:    generateHandles(100),
			expectedSeenCount: 0,
			assertSample: func(t *testing.T, reservoir []inventory.UpstreamHandle) {
				require.Empty(t, reservoir)
			},
		},
		{
			name:              "no agent to sample",
			filter:            filterOK,
			initialSeenCount:  0,
			handleFixtures:    generateHandles(0),
			expectedSeenCount: 0,
			assertSample: func(t *testing.T, reservoir []inventory.UpstreamHandle) {
				require.Empty(t, reservoir)
			},
		},
		{
			name:              "reservoir not filled entirely",
			filter:            filterOK,
			initialSeenCount:  0,
			handleFixtures:    generateHandles(5),
			expectedSeenCount: 5,
			assertSample: func(t *testing.T, reservoir []inventory.UpstreamHandle) {
				require.Len(t, reservoir, 5)
				for i, h := range reservoir {
					expectedHostname := fmt.Sprintf("hostname-%d", i)
					require.Equal(t, expectedHostname, h.Hello().Hostname)
				}
			},
		},
		{
			name:              "overflowing reservoir",
			filter:            filterOK,
			initialSeenCount:  0,
			handleFixtures:    generateHandles(110),
			expectedSeenCount: 110,
			assertSample: func(t *testing.T, reservoir []inventory.UpstreamHandle) {
				require.Len(t, reservoir, testSampleSize)
				// the probability of no elements being sampled in the last 100 elements is 10e-200
				require.NotElementsMatch(t, slices.Collect(generateHandles(testSampleSize)), reservoir)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sampler := handleSampler{
				sampleSize: testSampleSize,
				seenCount:  test.initialSeenCount,
				filter:     test.filter,
				sample:     make([]inventory.UpstreamHandle, 0, testSampleSize),
			}
			for handle := range test.handleFixtures {
				sampler.visit(handle)
			}

			require.Equal(t, test.expectedSeenCount, sampler.seenCount)
			test.assertSample(t, sampler.sample)
		})
	}
}
