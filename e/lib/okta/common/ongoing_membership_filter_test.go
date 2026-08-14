package common

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func TestOngoingAccessRequestMembershipFilter_Filter(t *testing.T) {
	const (
		finalized    = true
		notFinalized = false
	)
	tests := []struct {
		name                                   string
		inputOktaMembers                       map[string]*accesslist.AccessListMember
		inputTeleportMembers                   map[string]*accesslist.AccessListMember
		assignments                            []types.OktaAssignment
		excludeAssignmentProcessorRaces        bool
		expectedOktaMembers                    []string
		expectedTeleportMembers                []string
		disableAssignmentProcessorRacesFilters bool
	}{
		{
			name: "filters access-request assignment not in teleport members",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_PROCESSING),
			},
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "considers all the provided assignments",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
				"groupB/alice": newMember("groupB", "alice"),
				"groupC/alice": newMember("groupC", "alice"),
				"groupA/bob":   newMember("groupA", "bob"),
				"groupB/bob":   newMember("groupB", "bob"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_SUCCESSFUL),
				newAssignment("alice", "groupB", "access-request/abc", types.OktaAssignmentSpecV1_PROCESSING),
				newAssignment("bob", "groupA", "access-request/bcd", types.OktaAssignmentSpecV1_SUCCESSFUL),
				newAssignment("bob", "groupB", "access-request/cde", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			// access-request filter handles these, not the stale member filter.
			// The access-request filter also removes from inOkta when not in inTeleport.
			expectedOktaMembers:     []string{"groupC/alice"},
			expectedTeleportMembers: nil,
		},
		{
			name: "preserves access-request assignment if also in teleport members",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaMembers:     []string{"groupA/alice"},
			expectedTeleportMembers: []string{"groupA/alice"},
		},
		{
			name: "ignores unrelated assignment status",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/xyz", types.OktaAssignmentSpecV1_FAILED),
			},
			expectedOktaMembers:     []string{"groupA/alice"},
			expectedTeleportMembers: nil,
		},
		{
			name: "no assignments, no filtering",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers:    map[string]*accesslist.AccessListMember{},
			assignments:             []types.OktaAssignment{},
			expectedOktaMembers:     []string{"groupA/alice"},
			expectedTeleportMembers: nil,
		},
		{
			name: "pending assignment",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "assignment-processor", types.OktaAssignmentSpecV1_PENDING),
			},
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "cleanup assignment filters from both maps for non-finalized successful",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_SUCCESSFUL, notFinalized),
			},
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "cleanup assignment filters from both maps for non-finalized processing",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_PROCESSING, notFinalized),
			},
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "finalized cleanup assignment does not filter",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_SUCCESSFUL, finalized),
			},
			expectedOktaMembers:     []string{"groupA/alice"},
			expectedTeleportMembers: []string{"groupA/alice"},
		},
		{
			name: "assignment without cleanup time does not trigger cleanup filter",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "manual-sync", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "assignment without cleanup time does not trigger cleanup filter",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "manual-sync", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			// The staleOktaMemberFilter removes the Okta entry because the
			// assignment is successful+non-finalized and the member is not in Teleport.
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "stale okta member filter: successful assignment, member in okta but not teleport",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "user-assignment-creator", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "stale okta member filter: preserves member present in both okta and teleport",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "user-assignment-creator", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaMembers:     []string{"groupA/alice"},
			expectedTeleportMembers: []string{"groupA/alice"},
		},
		{
			name: "stale okta member filter: ignores finalized assignments",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_SUCCESSFUL, finalized),
			},
			expectedOktaMembers:     []string{"groupA/alice"},
			expectedTeleportMembers: nil,
		},
		{
			name: "stale okta member filter: ignores access-request assignments",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			// access-request filter handles these, not the stale member filter.
			// The access-request filter also removes from inOkta when not in inTeleport.
			expectedOktaMembers:     nil,
			expectedTeleportMembers: nil,
		},
		{
			name: "pending assignment filter should not trigger when on IncludeAssignmentProcessorRaces set to false",
			inputOktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			inputTeleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "assignment-processor", types.OktaAssignmentSpecV1_PENDING),
			},
			disableAssignmentProcessorRacesFilters: true,
			expectedOktaMembers:                    []string{"groupA/alice"},
			expectedTeleportMembers:                []string{"groupA/alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &OngoingAssignmentsMembershipFilter{
				AssignmentsService:              &mockAssignmentsService{assignments: tt.assignments},
				IncludeAssignmentProcessorRaces: !tt.disableAssignmentProcessorRacesFilters,
			}
			ctx := context.Background()

			inOkta := maps.Clone(tt.inputOktaMembers)
			inTeleport := maps.Clone(tt.inputTeleportMembers)
			err := filter.Filter(ctx, inOkta, inTeleport)
			require.NoError(t, err)

			require.Equal(t, tt.expectedOktaMembers, slices.Collect(maps.Keys(inOkta)), "unexpected Okta members")
			require.Equal(t, tt.expectedTeleportMembers, slices.Collect(maps.Keys(inTeleport)), "unexpected Teleport members")
		})
	}
}

type mockAssignmentsService struct {
	assignments []types.OktaAssignment
}

func (s *mockAssignmentsService) ListOktaAssignments(ctx context.Context, pageSize int, nextToken string) ([]types.OktaAssignment, string, error) {
	return s.assignments, "", nil
}

func newAssignment(user, group, source string, status types.OktaAssignmentSpecV1_OktaAssignmentStatus) types.OktaAssignment {
	return &types.OktaAssignmentV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   uuid.NewString(),
				Labels: map[string]string{eteleport.OktaAssignmentSourceLabel: source},
			},
		},
		Spec: types.OktaAssignmentSpecV1{
			User:    user,
			Status:  status,
			Targets: []*types.OktaAssignmentTargetV1{{Id: group, Type: types.OktaAssignmentTargetV1_GROUP}},
		},
	}
}

func newCleanupAssignment(user, group string, status types.OktaAssignmentSpecV1_OktaAssignmentStatus, finalized bool) types.OktaAssignment {
	return &types.OktaAssignmentV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: uuid.NewString(),
			},
		},
		Spec: types.OktaAssignmentSpecV1{
			User:        user,
			Status:      status,
			Finalized:   finalized,
			CleanupTime: time.Now(),
			Targets:     []*types.OktaAssignmentTargetV1{{Id: group, Type: types.OktaAssignmentTargetV1_GROUP}},
		},
	}
}

func newMember(group, user string) *accesslist.AccessListMember {
	return &accesslist.AccessListMember{
		Spec: accesslist.AccessListMemberSpec{AccessList: group},
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: user},
		},
	}
}
