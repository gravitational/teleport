package auth

import (
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
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

	const testNodeCount = 1000
	const testGroupName = "my-group"
	testGroups := []string{testGroupName, "catch-all"}

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
	for _, canary := range canaries {
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

	// TODO: write tests for the catch-all behaviour
}

// TODO: write test for agent lookup
