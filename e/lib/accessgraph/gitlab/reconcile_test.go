package gitlab

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestReconcileResults(t *testing.T) {
	oldUsers := []*accessgraphv1alpha.GitlabUser{
		{Username: "userA"},
		{Username: "userB"},
	}
	newUsers := []*accessgraphv1alpha.GitlabUser{
		{Username: "userA"},
		{Username: "userC"},
	}

	oldGroups := []*accessgraphv1alpha.GitlabGroup{
		{Path: "groupA"},
		{Path: "groupB"},
	}
	newGroups := []*accessgraphv1alpha.GitlabGroup{
		{Path: "groupA"},
		{Path: "groupC"},
	}

	oldProjects := []*accessgraphv1alpha.GitlabProject{
		{Path: "projectA"},
		{Path: "projectB"},
	}
	newProjects := []*accessgraphv1alpha.GitlabProject{
		{Path: "projectA"},
		{Path: "projectC"},
	}

	oldProjectMembers := []*accessgraphv1alpha.GitlabProjectMember{
		{Username: "projectMemberA", Project: &accessgraphv1alpha.GitlabProject{Path: "projectA"}},
		{Username: "projectMemberB", Project: &accessgraphv1alpha.GitlabProject{Path: "projectA"}},
	}
	newProjectMembers := []*accessgraphv1alpha.GitlabProjectMember{
		{Username: "projectMemberA", Project: &accessgraphv1alpha.GitlabProject{Path: "projectA"}},
		{Username: "projectMemberC", Project: &accessgraphv1alpha.GitlabProject{Path: "projectA"}},
	}

	oldGroupMembers := []*accessgraphv1alpha.GitlabGroupMember{
		{Username: "groupMemberA", Group: &accessgraphv1alpha.GitlabGroup{Path: "groupA"}},
		{Username: "groupMemberB", Group: &accessgraphv1alpha.GitlabGroup{Path: "groupA"}},
	}
	newGroupMembers := []*accessgraphv1alpha.GitlabGroupMember{
		{Username: "groupMemberA", Group: &accessgraphv1alpha.GitlabGroup{Path: "groupA"}},
		{Username: "groupMemberC", Group: &accessgraphv1alpha.GitlabGroup{Path: "groupA"}},
	}

	oldResources := &resources{
		Users:          oldUsers,
		Groups:         oldGroups,
		Projects:       oldProjects,
		ProjectMembers: oldProjectMembers,
		GroupMembers:   oldGroupMembers,
	}

	newResources := &resources{
		Users:          newUsers,
		Groups:         newGroups,
		Projects:       newProjects,
		ProjectMembers: newProjectMembers,
		GroupMembers:   newGroupMembers,
	}

	upsert, delete := reconcileResults(oldResources, newResources)

	wantUpsert := &accessgraphv1alpha.GitlabResourceList{
		Resources: []*accessgraphv1alpha.GitlabResource{
			{Resource: &accessgraphv1alpha.GitlabResource_User{User: newUsers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Group{Group: newGroups[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Project{Project: newProjects[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{ProjectMember: newProjectMembers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_GroupMember{GroupMember: newGroupMembers[1]}},
		},
	}

	wantDelete := &accessgraphv1alpha.GitlabResourceList{
		Resources: []*accessgraphv1alpha.GitlabResource{
			{Resource: &accessgraphv1alpha.GitlabResource_User{User: oldUsers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Group{Group: oldGroups[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Project{Project: oldProjects[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{ProjectMember: oldProjectMembers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_GroupMember{GroupMember: oldGroupMembers[1]}},
		},
	}

	require.Empty(t, cmp.Diff(
		wantUpsert.Resources, upsert.Resources,
		protocmp.Transform(),
	),
	)
	require.Empty(t, cmp.Diff(
		wantDelete.Resources, delete.Resources,
		protocmp.Transform(),
	),
	)

}
