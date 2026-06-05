package gitlab

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// reconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two GitlabResourceList objects, one for resources to upsert and one
// for resources to delete.
func reconcileResults(old *resources, new *resources) (upsert, delete *accessgraphv1alpha.GitlabResourceList) {
	upsert, delete = &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	if old == nil {
		old = &resources{}
	}
	if new == nil {
		new = &resources{}
	}

	for _, results := range []*reconcileIntermediateResult{
		reconcileUsers(old.Users, new.Users),
		reconcileGroups(old.Groups, new.Groups),
		reconcileProjects(old.Projects, new.Projects),
		reconcileProjectMembers(old.ProjectMembers, new.ProjectMembers),
		reconcileGroupMembers(old.GroupMembers, new.GroupMembers),
	} {
		upsert.SetResources(append(upsert.GetResources(), results.upsert.GetResources()...))
		delete.SetResources(append(delete.GetResources(), results.delete.GetResources()...))
	}

	return upsert, delete
}

type reconcileIntermediateResult struct {
	upsert, delete *accessgraphv1alpha.GitlabResourceList
}

func reconcileGroups(old []*accessgraphv1alpha.GitlabGroup, new []*accessgraphv1alpha.GitlabGroup) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(group *accessgraphv1alpha.GitlabGroup) string {
		return group.GetPath()
	})

	for _, group := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Group{
				Group: group,
			},
		}))
	}
	for _, group := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Group{
				Group: group,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileUsers(
	old []*accessgraphv1alpha.GitlabUser,
	new []*accessgraphv1alpha.GitlabUser,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(user *accessgraphv1alpha.GitlabUser) string {
		return user.GetUsername()
	})
	for _, user := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_User{
				User: user,
			},
		}))
	}
	for _, user := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_User{
				User: user,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileProjects(
	old []*accessgraphv1alpha.GitlabProject,
	new []*accessgraphv1alpha.GitlabProject,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(project *accessgraphv1alpha.GitlabProject) string {
		return project.GetPath()
	})
	for _, project := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Project{
				Project: project,
			},
		}))
	}
	for _, project := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Project{
				Project: project,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileProjectMembers(
	old []*accessgraphv1alpha.GitlabProjectMember,
	new []*accessgraphv1alpha.GitlabProjectMember,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.GitlabProjectMember) string {
		return fmt.Sprintf("%x;%x", policy.GetUsername(), policy.GetProject().GetPath())
	})
	for _, projectMember := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{
				ProjectMember: projectMember,
			},
		}))
	}
	for _, projectMember := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{
				ProjectMember: projectMember,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileGroupMembers(
	old []*accessgraphv1alpha.GitlabGroupMember,
	new []*accessgraphv1alpha.GitlabGroupMember,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.GitlabGroupMember) string {
		return fmt.Sprintf("%x;%x", policy.GetUsername(), policy.GetGroup().GetPath())
	})
	for _, member := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_GroupMember{
				GroupMember: member,
			},
		}))
	}
	for _, member := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_GroupMember{
				GroupMember: member,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcile[T protoreflect.ProtoMessage](old []T, new []T, key func(T) string) (upsert, delete []T) {
	if len(old) == 0 {
		return new, nil
	}
	if len(new) == 0 {
		return nil, old
	}

	oldMap := make(map[string]T, len(old))
	for _, item := range old {
		oldMap[key(item)] = item
	}

	newMap := make(map[string]T, len(new))
	for _, item := range new {
		newMap[key(item)] = item
	}

	for _, item := range new {
		if oldItem, ok := oldMap[key(item)]; !ok || !proto.Equal(oldItem, item) {
			upsert = append(upsert, item)
		}
	}
	for _, item := range old {
		if _, ok := newMap[key(item)]; !ok {
			delete = append(delete, item)
		}
	}
	return upsert, delete
}
