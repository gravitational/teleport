package github

import (
	"fmt"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func reconcileResults(old *pollResults, new *pollResults) (upsert, delete *accessgraphv1alpha.GithubResourceList) {
	upsert, delete = &accessgraphv1alpha.GithubResourceList{}, &accessgraphv1alpha.GithubResourceList{}

	if old == nil {
		old = &pollResults{}
	}
	if new == nil {
		new = &pollResults{}
	}

	for _, results := range []*reconcileIntermediateResult{
		reconcileTokens(old.tokens, new.tokens),
		reconcileRoleAssignment(old.roleAssignments, new.roleAssignments),
		reconcileRepositories(old.repos, new.repos),
		reconcileRoles(old.roles, new.roles),
	} {
		upsert.SetResources(append(upsert.GetResources(), results.upsert.GetResources()...))
		delete.SetResources(append(delete.GetResources(), results.delete.GetResources()...))
	}

	return upsert, delete
}

type reconcileIntermediateResult struct {
	upsert, delete *accessgraphv1alpha.GithubResourceList
}

func reconcileRoles(old []*accessgraphv1alpha.GithubRoleV1, new []*accessgraphv1alpha.GithubRoleV1) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GithubResourceList{}, &accessgraphv1alpha.GithubResourceList{}

	toAdd, toRemove := reconcile(old, new, func(group *accessgraphv1alpha.GithubRoleV1) string {
		return group.GetName()
	})

	for _, group := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.GithubResource_builder{
			Role: proto.ValueOrDefault(group),
		}.Build()))
	}
	for _, group := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.GithubResource_builder{
			Role: proto.ValueOrDefault(group),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileTokens(
	old []*accessgraphv1alpha.GithubTokenV1,
	new []*accessgraphv1alpha.GithubTokenV1,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GithubResourceList{}, &accessgraphv1alpha.GithubResourceList{}

	toAdd, toRemove := reconcile(old, new, func(token *accessgraphv1alpha.GithubTokenV1) string {
		return strconv.FormatInt(token.GetId(), 10)
	})
	for _, token := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.GithubResource_builder{
			Token: proto.ValueOrDefault(token),
		}.Build()))
	}
	for _, token := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.GithubResource_builder{
			Token: proto.ValueOrDefault(token),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRepositories(
	old []*accessgraphv1alpha.GithubRepositoryV1,
	new []*accessgraphv1alpha.GithubRepositoryV1,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GithubResourceList{}, &accessgraphv1alpha.GithubResourceList{}

	toAdd, toRemove := reconcile(old, new, func(project *accessgraphv1alpha.GithubRepositoryV1) string {
		return project.GetName()
	})
	for _, project := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.GithubResource_builder{
			Repository: proto.ValueOrDefault(project),
		}.Build()))
	}
	for _, project := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.GithubResource_builder{
			Repository: proto.ValueOrDefault(project),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRoleAssignment(
	old []*accessgraphv1alpha.GithubRoleAssignmentV1,
	new []*accessgraphv1alpha.GithubRoleAssignmentV1,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.GithubResourceList{}, &accessgraphv1alpha.GithubResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.GithubRoleAssignmentV1) string {
		return fmt.Sprintf("%x;%x;%v", policy.GetUser(), policy.GetRoleId(), policy.GetOwner())
	})
	for _, member := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.GithubResource_builder{
			RoleAssignment: proto.ValueOrDefault(member),
		}.Build()))
	}
	for _, member := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.GithubResource_builder{
			RoleAssignment: proto.ValueOrDefault(member),
		}.Build()))
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
