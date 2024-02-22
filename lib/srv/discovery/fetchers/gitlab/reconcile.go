/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package gitlab

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// ReconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two AWSResourceList objects, one for resources to upsert and one
// for resources to delete.
func ReconcileResults(old *Resources, new *Resources) (upsert, delete *accessgraphv1alpha.GitlabResourceList) {
	upsert, delete = &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	for _, results := range []*reconcileIntermeditateResult{
		reconcileGroups(old.Groups, new.Groups),
		reconcileGroupMembers(old.GroupMembers, new.GroupMembers),
		reconcileProjects(old.Projects, new.Projects),
		reconcileProjectMembers(old.ProjectMembers, new.ProjectMembers),
	} {
		upsert.Resources = append(upsert.Resources, results.upsert.Resources...)
		delete.Resources = append(delete.Resources, results.delete.Resources...)
	}

	return upsert, delete
}

type reconcileIntermeditateResult struct {
	upsert, delete *accessgraphv1alpha.GitlabResourceList
}

func reconcileGroups(old []*accessgraphv1alpha.GitlabGroup, new []*accessgraphv1alpha.GitlabGroup) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(g *accessgraphv1alpha.GitlabGroup) string {
		return fmt.Sprintf("%s;%s", g.Path, g.Name)
	})

	for _, instance := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Group{
				Group: instance,
			},
		})
	}
	for _, instance := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Group{
				Group: instance,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileGroupMembers(
	old []*accessgraphv1alpha.GitlabGroupMember,
	new []*accessgraphv1alpha.GitlabGroupMember,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(user *accessgraphv1alpha.GitlabGroupMember) string {
		return fmt.Sprintf("%s;%s", user.Username, user.Group.Path)
	})
	for _, user := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_GroupMember{
				GroupMember: user,
			},
		})
	}
	for _, user := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_GroupMember{
				GroupMember: user,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileProjects(
	old []*accessgraphv1alpha.GitlabProject,
	new []*accessgraphv1alpha.GitlabProject,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(project *accessgraphv1alpha.GitlabProject) string {
		return fmt.Sprintf("%s", project.Path)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Project{
				Project: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_Project{
				Project: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileProjectMembers(
	old []*accessgraphv1alpha.GitlabProjectMember,
	new []*accessgraphv1alpha.GitlabProjectMember,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.GitlabResourceList{}, &accessgraphv1alpha.GitlabResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.GitlabProjectMember) string {
		return fmt.Sprintf("%s;%s", policy.Project.Path, policy.Username)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{
				ProjectMember: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.GitlabResource{
			Resource: &accessgraphv1alpha.GitlabResource_ProjectMember{
				ProjectMember: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
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
