package service

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// reconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two NetIQResourceList objects, one for resources to upsert and one
// for resources to delete.
func reconcileResults(old *resources, new *resources) (upsert, delete *accessgraphv1alpha.NetIQResourceList) {
	upsert, delete = &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	if old == nil {
		old = &resources{}
	}
	if new == nil {
		new = &resources{}
	}

	for _, results := range []*reconcileIntermediateResult{
		reconcileUsers(old.users, new.users),
		reconcileGroups(old.groups, new.groups),
		reconcileRoles(old.roles, new.roles),
		reconcileGroupMembers(old.groupMembers, new.groupMembers),
		reconcileRoleMembers(old.roleMembers, new.roleMembers),
		reconcileRoleMappedResources(old.mappedResources, new.mappedResources),
		reconcileRoleParentRoles(old.parentRoles, new.parentRoles),
		reconcileResources(old.resources, new.resources),
	} {
		upsert.SetResources(append(upsert.GetResources(), results.upsert.GetResources()...))
		delete.SetResources(append(delete.GetResources(), results.delete.GetResources()...))
	}

	return upsert, delete
}

type reconcileIntermediateResult struct {
	upsert, delete *accessgraphv1alpha.NetIQResourceList
}

func reconcileGroups(old []*accessgraphv1alpha.NetIQGroup, new []*accessgraphv1alpha.NetIQGroup) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(group *accessgraphv1alpha.NetIQGroup) string {
		return group.GetId()
	})

	for _, group := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			Group: proto.ValueOrDefault(group),
		}.Build()))
	}
	for _, group := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			Group: proto.ValueOrDefault(group),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileUsers(
	old []*accessgraphv1alpha.NetIQUser,
	new []*accessgraphv1alpha.NetIQUser,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(user *accessgraphv1alpha.NetIQUser) string {
		return user.GetId()
	})
	for _, user := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			User: proto.ValueOrDefault(user),
		}.Build()))
	}
	for _, user := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			User: proto.ValueOrDefault(user),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRoles(
	old []*accessgraphv1alpha.NetIQRole,
	new []*accessgraphv1alpha.NetIQRole,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(role *accessgraphv1alpha.NetIQRole) string {
		return role.GetId()
	})
	for _, role := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			Role: proto.ValueOrDefault(role),
		}.Build()))
	}
	for _, role := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			Role: proto.ValueOrDefault(role),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRoleMembers(
	old []*accessgraphv1alpha.NetIQMemberAssignmentRef,
	new []*accessgraphv1alpha.NetIQMemberAssignmentRef,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(memberAssign *accessgraphv1alpha.NetIQMemberAssignmentRef) string {
		return fmt.Sprintf("%x;%x", memberAssign.GetRoleId(), memberAssign.GetDn())
	})
	for _, roleMember := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			RoleMemberRef: proto.ValueOrDefault(roleMember),
		}.Build()))
	}
	for _, roleMember := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			RoleMemberRef: proto.ValueOrDefault(roleMember),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileGroupMembers(
	old []*accessgraphv1alpha.NetIQGroupMember,
	new []*accessgraphv1alpha.NetIQGroupMember,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(groupMember *accessgraphv1alpha.NetIQGroupMember) string {
		return fmt.Sprintf("%x;%x", groupMember.GetGroupId(), groupMember.GetUserId())
	})
	for _, member := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			GroupMember: proto.ValueOrDefault(member),
		}.Build()))
	}
	for _, member := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			GroupMember: proto.ValueOrDefault(member),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileResources(
	old []*accessgraphv1alpha.NetIQResource,
	new []*accessgraphv1alpha.NetIQResource,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(resource *accessgraphv1alpha.NetIQResource) string {
		return resource.GetId()
	})
	for _, resource := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			Resource: proto.ValueOrDefault(resource),
		}.Build()))
	}
	for _, resource := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			Resource: proto.ValueOrDefault(resource),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRoleMappedResources(
	old []*accessgraphv1alpha.NetIQResourceAssignmentRef,
	new []*accessgraphv1alpha.NetIQResourceAssignmentRef,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(resourceAssign *accessgraphv1alpha.NetIQResourceAssignmentRef) string {
		return fmt.Sprintf("%x;%x", resourceAssign.GetRoleId(), resourceAssign.GetResourceId())
	})
	for _, ref := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			ResourceRoleRef: proto.ValueOrDefault(ref),
		}.Build()))
	}
	for _, ref := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			ResourceRoleRef: proto.ValueOrDefault(ref),
		}.Build()))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRoleParentRoles(
	old []*accessgraphv1alpha.NetIQRoleRef,
	new []*accessgraphv1alpha.NetIQRoleRef,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.NetIQResourceList{}, &accessgraphv1alpha.NetIQResourceList{}

	toAdd, toRemove := reconcile(old, new, func(roleRef *accessgraphv1alpha.NetIQRoleRef) string {
		return fmt.Sprintf("%x;%x", roleRef.GetChildRoleId(), roleRef.GetParentRoleId())
	})
	for _, ref := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			ParentRoleRef: proto.ValueOrDefault(ref),
		}.Build()))
	}
	for _, ref := range toRemove {
		delete.SetResources(append(delete.GetResources(), accessgraphv1alpha.NetIQObject_builder{
			ParentRoleRef: proto.ValueOrDefault(ref),
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
