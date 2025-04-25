package common

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func TestOngoingAccessRequestMembershipFilter_Filter(t *testing.T) {
	tests := []struct {
		name            string
		oktaMembers     map[string]*accesslist.AccessListMember
		teleportMembers map[string]*accesslist.AccessListMember
		assignments     []types.OktaAssignment
		expectedPresent bool
	}{
		{
			name: "filters access-request assignment not in teleport members",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": {
					Spec: accesslist.AccessListMemberSpec{AccessList: "groupA"},
					ResourceHeader: header.ResourceHeader{
						Metadata: header.Metadata{Name: "alice"},
					},
				},
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_PROCESSING),
			},
			expectedPresent: false,
		},
		{
			name: "preserves access-request assignment if also in teleport members",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": {
					Spec: accesslist.AccessListMemberSpec{AccessList: "groupA"},
					ResourceHeader: header.ResourceHeader{
						Metadata: header.Metadata{Name: "alice"},
					},
				},
			},
			teleportMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": {
					Spec: accesslist.AccessListMemberSpec{AccessList: "groupA"},
					ResourceHeader: header.ResourceHeader{
						Metadata: header.Metadata{Name: "alice"},
					},
				},
			},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/abc", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedPresent: true,
		},
		{
			name: "ignores non-access-request assignment",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": {
					Spec: accesslist.AccessListMemberSpec{AccessList: "groupA"},
					ResourceHeader: header.ResourceHeader{
						Metadata: header.Metadata{Name: "alice"},
					},
				},
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "manual-sync", types.OktaAssignmentSpecV1_SUCCESSFUL),
			},
			expectedPresent: true,
		},
		{
			name: "ignores unrelated assignment status",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": {
					Spec: accesslist.AccessListMemberSpec{AccessList: "groupA"},
					ResourceHeader: header.ResourceHeader{
						Metadata: header.Metadata{Name: "alice"},
					},
				},
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments: []types.OktaAssignment{
				newAssignment("alice", "groupA", "access-request/xyz", types.OktaAssignmentSpecV1_FAILED),
			},
			expectedPresent: true,
		},
		{
			name: "no assignments, no filtering",
			oktaMembers: map[string]*accesslist.AccessListMember{
				"groupA/alice": {
					Spec: accesslist.AccessListMemberSpec{AccessList: "groupA"},
					ResourceHeader: header.ResourceHeader{
						Metadata: header.Metadata{Name: "alice"},
					},
				},
			},
			teleportMembers: map[string]*accesslist.AccessListMember{},
			assignments:     []types.OktaAssignment{},
			expectedPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &OngoingAccessRequestMembershipFilter{
				AssignmentsService: &mockAssignmentsService{assignments: tt.assignments},
			}
			ctx := context.Background()

			filtered, err := filter.Filter(ctx, tt.oktaMembers, tt.teleportMembers)
			require.NoError(t, err)

			_, present := filtered["groupA/alice"]
			require.Equal(t, tt.expectedPresent, present)
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
