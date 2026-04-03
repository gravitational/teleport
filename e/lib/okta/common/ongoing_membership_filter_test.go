package common

import (
	"context"
	"maps"
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
		name                    string
		oktaMembers             map[string]*accesslist.AccessListMember
		teleportMembers         map[string]*accesslist.AccessListMember
		assignments             []types.OktaAssignment
		expectedOktaPresent     bool
		expectedTeleportPresent bool
	}{
		{
			name: "filters access-request assignment not in teleport members",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_PROCESSING),
			},
			expectedOktaPresent:     false,
			expectedTeleportPresent: false,
		},
		{
			name: "preserves access-request assignment if also in teleport members",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaPresent:     true,
			expectedTeleportPresent: true,
		},
		{
			name: "ignores non-access-request assignment",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "manual-sync", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaPresent:     true,
			expectedTeleportPresent: false,
		},
		{
			name: "ignores unrelated assignment status",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/xyz", types.OktaAssignmentSpecV1_FAILED),
			},
			expectedOktaPresent:     true,
			expectedTeleportPresent: false,
		},
		{
			name: "no assignments, no filtering",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers:         map[string]*accesslist.AccessListMember{},
			assignments:             []types.OktaAssignment{},
			expectedOktaPresent:     true,
			expectedTeleportPresent: false,
		},
		{
			name: "pending assignment",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "assignment-processor", types.OktaAssignmentSpecV1_PENDING),
			},
			expectedOktaPresent:     false,
			expectedTeleportPresent: false,
		},
		{
			name: "cleanup assignment filters from both maps for non-finalized successful",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_SUCCESSFUL, notFinalized),
			},
			expectedOktaPresent:     false,
			expectedTeleportPresent: false,
		},
		{
			name: "cleanup assignment filters from both maps for non-finalized processing",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_PROCESSING, notFinalized),
			},
			expectedOktaPresent:     false,
			expectedTeleportPresent: false,
		},
		{
			name: "finalized cleanup assignment does not filter",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			assignments: []types.OktaAssignment{
				newCleanupAssignment("alice", "groupA", types.OktaAssignmentSpecV1_SUCCESSFUL, finalized),
			},
			expectedOktaPresent:     true,
			expectedTeleportPresent: true,
		},
		{
			name: "assignment without cleanup time does not trigger cleanup filter",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": newMember("groupA", "alice"),
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "manual-sync", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedOktaPresent:     true,
			expectedTeleportPresent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &OngoingAssignmentsMembershipFilter{
				AssignmentsService: &mockAssignmentsService{assignments: tt.assignments},
			}
			ctx := context.Background()

			inOkta := maps.Clone(tt.oktaMembers)
			inTeleport := maps.Clone(tt.teleportMembers)
			err := filter.Filter(ctx, inOkta, inTeleport)
			require.NoError(t, err)

			_, oktaPresent := inOkta["groupA/alice"]
			require.Equal(t, tt.expectedOktaPresent, oktaPresent, "unexpected presence in oktaMembers")

			_, teleportPresent := inTeleport["groupA/alice"]
			require.Equal(t, tt.expectedTeleportPresent, teleportPresent, "unexpected presence in teleportMembers")
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
