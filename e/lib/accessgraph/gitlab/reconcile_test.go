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
		accessgraphv1alpha.GitlabUser_builder{Username: "userA"}.Build(),
		accessgraphv1alpha.GitlabUser_builder{Username: "userB"}.Build(),
	}
	newUsers := []*accessgraphv1alpha.GitlabUser{
		accessgraphv1alpha.GitlabUser_builder{Username: "userA"}.Build(),
		accessgraphv1alpha.GitlabUser_builder{Username: "userC"}.Build(),
	}

	oldGroups := []*accessgraphv1alpha.GitlabGroup{
		accessgraphv1alpha.GitlabGroup_builder{Path: "groupA"}.Build(),
		accessgraphv1alpha.GitlabGroup_builder{Path: "groupB"}.Build(),
	}
	newGroups := []*accessgraphv1alpha.GitlabGroup{
		accessgraphv1alpha.GitlabGroup_builder{Path: "groupA"}.Build(),
		accessgraphv1alpha.GitlabGroup_builder{Path: "groupC"}.Build(),
	}

	oldProjects := []*accessgraphv1alpha.GitlabProject{
		accessgraphv1alpha.GitlabProject_builder{Path: "projectA"}.Build(),
		accessgraphv1alpha.GitlabProject_builder{Path: "projectB"}.Build(),
	}
	newProjects := []*accessgraphv1alpha.GitlabProject{
		accessgraphv1alpha.GitlabProject_builder{Path: "projectA"}.Build(),
		accessgraphv1alpha.GitlabProject_builder{Path: "projectC"}.Build(),
	}

	oldProjectMembers := []*accessgraphv1alpha.GitlabProjectMember{
		accessgraphv1alpha.GitlabProjectMember_builder{Username: "projectMemberA", Project: accessgraphv1alpha.GitlabProject_builder{Path: "projectA"}.Build()}.Build(),
		accessgraphv1alpha.GitlabProjectMember_builder{Username: "projectMemberB", Project: accessgraphv1alpha.GitlabProject_builder{Path: "projectA"}.Build()}.Build(),
	}
	newProjectMembers := []*accessgraphv1alpha.GitlabProjectMember{
		accessgraphv1alpha.GitlabProjectMember_builder{Username: "projectMemberA", Project: accessgraphv1alpha.GitlabProject_builder{Path: "projectA"}.Build()}.Build(),
		accessgraphv1alpha.GitlabProjectMember_builder{Username: "projectMemberC", Project: accessgraphv1alpha.GitlabProject_builder{Path: "projectA"}.Build()}.Build(),
	}

	oldGroupMembers := []*accessgraphv1alpha.GitlabGroupMember{
		accessgraphv1alpha.GitlabGroupMember_builder{Username: "groupMemberA", Group: accessgraphv1alpha.GitlabGroup_builder{Path: "groupA"}.Build()}.Build(),
		accessgraphv1alpha.GitlabGroupMember_builder{Username: "groupMemberB", Group: accessgraphv1alpha.GitlabGroup_builder{Path: "groupA"}.Build()}.Build(),
	}
	newGroupMembers := []*accessgraphv1alpha.GitlabGroupMember{
		accessgraphv1alpha.GitlabGroupMember_builder{Username: "groupMemberA", Group: accessgraphv1alpha.GitlabGroup_builder{Path: "groupA"}.Build()}.Build(),
		accessgraphv1alpha.GitlabGroupMember_builder{Username: "groupMemberC", Group: accessgraphv1alpha.GitlabGroup_builder{Path: "groupA"}.Build()}.Build(),
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

	wantUpsert := accessgraphv1alpha.GitlabResourceList_builder{
		Resources: []*accessgraphv1alpha.GitlabResource{
			{Resource: &accessgraphv1alpha.GitlabResource_User{User: newUsers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Group{Group: newGroups[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Project{Project: newProjects[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{ProjectMember: newProjectMembers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_GroupMember{GroupMember: newGroupMembers[1]}},
		},
	}.Build()

	wantDelete := accessgraphv1alpha.GitlabResourceList_builder{
		Resources: []*accessgraphv1alpha.GitlabResource{
			{Resource: &accessgraphv1alpha.GitlabResource_User{User: oldUsers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Group{Group: oldGroups[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_Project{Project: oldProjects[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{ProjectMember: oldProjectMembers[1]}},
			{Resource: &accessgraphv1alpha.GitlabResource_GroupMember{GroupMember: oldGroupMembers[1]}},
		},
	}.Build()

	require.Empty(t, cmp.Diff(
		wantUpsert.GetResources(), upsert.GetResources(),
		protocmp.Transform(),
	),
	)
	require.Empty(t, cmp.Diff(
		wantDelete.GetResources(), delete.GetResources(),
		protocmp.Transform(),
	),
	)

}
