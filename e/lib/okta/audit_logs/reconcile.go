package auditlogs

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func reconcileResults(old *pollResults, new *pollResults) (upsert, delete *accessgraphv1alpha.OktaResourceList) {
	upsert, delete = &accessgraphv1alpha.OktaResourceList{}, &accessgraphv1alpha.OktaResourceList{}

	if old == nil {
		old = &pollResults{}
	}
	if new == nil {
		new = &pollResults{}
	}

	for _, results := range []*reconcileIntermediateResult{
		reconcileTokens(old.tokens, new.tokens),
		reconcileRoleAssignment(old.roleAssignments, new.roleAssignments),
		reconcileRoles(old.roles, new.roles),
	} {
		upsert.SetResources(append(upsert.GetResources(), results.upsert.GetResources()...))
		delete.SetResources(append(delete.GetResources(), results.delete.GetResources()...))
	}

	return upsert, delete
}

type reconcileIntermediateResult struct {
	upsert, delete *accessgraphv1alpha.OktaResourceList
}

func reconcileRoles(old, new []*accessgraphv1alpha.OktaRoleV1) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.OktaResourceList{}, &accessgraphv1alpha.OktaResourceList{}

	toAdd, toRemove := reconcile(old, new, func(group *accessgraphv1alpha.OktaRoleV1) string {
		return group.GetRoleId()
	})

	for _, group := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.OktaResource{
			Resource: &accessgraphv1alpha.OktaResource_Role{
				Role: group,
			},
		}))
	}
	for _, group := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.OktaResource{
			Resource: &accessgraphv1alpha.OktaResource_Role{
				Role: group,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileTokens(
	old, new []*accessgraphv1alpha.OktaTokenV1,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.OktaResourceList{}, &accessgraphv1alpha.OktaResourceList{}

	toAdd, toRemove := reconcile(old, new, func(token *accessgraphv1alpha.OktaTokenV1) string {
		return token.GetId()
	})
	for _, token := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.OktaResource{
			Resource: &accessgraphv1alpha.OktaResource_Token{
				Token: token,
			},
		}))
	}
	for _, token := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.OktaResource{
			Resource: &accessgraphv1alpha.OktaResource_Token{
				Token: token,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcileRoleAssignment(
	old, new []*accessgraphv1alpha.OktaRoleAssignmentV1,
) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.OktaResourceList{}, &accessgraphv1alpha.OktaResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.OktaRoleAssignmentV1) string {
		return fmt.Sprintf("%x;%x;%v", policy.GetRoleId(), policy.GetUserId(), policy.GetOrganization())
	})
	for _, member := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.OktaResource{
			Resource: &accessgraphv1alpha.OktaResource_RoleAssignment{
				RoleAssignment: member,
			},
		}))
	}
	for _, member := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.OktaResource{
			Resource: &accessgraphv1alpha.OktaResource_RoleAssignment{
				RoleAssignment: member,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcile[T proto.Message](old, new []T, key func(T) string) (upsert, delete []T) {
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
