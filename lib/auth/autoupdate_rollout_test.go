package auth

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"testing"
	"time"
)

func TestSampleAgentsFromGroup(t *testing.T) {
	clock := clockwork.NewFakeClock()

	auth := &Server{
		clock:    clock,
		ServerID: uuid.NewString(),
		logger:   utils.NewSlogLoggerForTests(),
	}
	auth.Cache = auth.Services
	controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
	auth.inventory = controller

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
	canariesCatchAll, err := auth.SampleAgentsFromAutoUpdateGroup(t.Context(), testGroupName, sampleSize, []string{"group-a", testCatchAllGroupName})
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

	auth := &Server{
		clock:    clock,
		ServerID: uuid.NewString(),
		logger:   utils.NewSlogLoggerForTests(),
	}
	auth.Cache = auth.Services
	controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
	auth.inventory = controller

	const testNodeCount = 5
	const testGroupName = "my-group"

	hosts := make(map[string][]*proto.UpstreamInventoryHello, testNodeCount)

	// Test setup:
	// We register X nodes, node number 1 has 1 handler, node 2 has 2, ...
	for i := 1; i <= testNodeCount; i++ {
		hostID := uuid.New().String()
		updaterID := uuid.New()
		hellos := make([]*proto.UpstreamInventoryHello, i)
		for j := range i {
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
