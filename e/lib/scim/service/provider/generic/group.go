package generic

import (
	"context"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/lister"
	libaccesslist "github.com/gravitational/teleport/lib/accesslists"
)

type groupHandler struct {
	common.Config
	Plugin *types.PluginV1
}

// CreateResource handles the creation of a new SCIM group resource.
func (g groupHandler) CreateResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	displayName, err := conv.GetGroupDisplayName(req.GetResource())
	if err != nil {
		return nil, trace.Wrap(err, "failed to get group display name")
	}

	accessList, err := g.findAccessListByTitle(ctx, displayName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, members, err := conv.AccessListFromResource(
		req.GetResource(),
		conv.WithClock(g.Clock),
		conv.WithAccessListName(accessList.GetName()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.AccessListToResource(accessList, members)
}

func (g groupHandler) findAccessListByTitle(ctx context.Context, title string) (*accesslist.AccessList, error) {
	for list, err := range clientutils.Resources(ctx, g.AccessListsService.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !accessListPredicate(list) {
			continue
		}

		if list.Spec.Title == title {
			return list, nil
		}
	}

	return nil, trace.NotFound(
		"Access List with tile %q does not exist. "+
			"To represent this as a SCIM group, a corresponding Access List with the same Title name must be created in Teleport.",
		title,
	)
}

// ListResources lists all SCIM group resources.
func (g groupHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	l := lister.GroupLister{
		Config:    g.Config,
		Predicate: func(list *accesslist.AccessList) bool { return accessListPredicate(list) },
		AccessListToResource: func(list *accesslist.AccessList) (*scimpb.Resource, error) {
			members, err := libaccesslist.GetMembersFor(ctx, list.GetName(), g.AccessListsService)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conv.AccessListToResource(list, members)
		},
	}

	return l.ListResources(ctx, req)
}

// GetResource retrieves a specific SCIM group resource by its ID.
func (g groupHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	resourceID := req.GetTarget().GetResourceId()

	acl, err := g.AccessListGetter.GetAccessList(ctx, resourceID)
	if err != nil {
		return nil, trace.Wrap(err, "fetching ACL for provisioning")
	}

	if !accessListPredicate(acl) {
		return nil, trace.NotFound("access list %q not found", resourceID)
	}

	members, err := libaccesslist.GetMembersFor(ctx, resourceID, g.AccessListsService)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.AccessListToResource(acl, members)
}

// UpdateResource handles the update of an existing SCIM group resource.
func (g groupHandler) UpdateResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	resourceID := req.GetTarget().GetResourceId()

	acl, err := g.AccessListGetter.GetAccessList(ctx, resourceID)
	if err != nil {
		return nil, trace.Wrap(err, "fetching ACL for provisioning")
	}

	if !accessListPredicate(acl) {
		return nil, trace.NotFound("access list %q not found", resourceID)
	}

	updatedACL, members, err := conv.AccessListFromResource(
		req.GetResource(),
		conv.WithClock(g.Clock),
		conv.WithMemberAddedBy("SCIM"),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Update title; fallback to current revision if missing
	acl.Spec.Title = updatedACL.Spec.Title
	if acl.GetRevision() == "" {
		acl.SetRevision(acl.GetRevision())
	}

	newACL, newMembers, err := g.AccessListsService.UpsertAccessListWithMembers(ctx, acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.AccessListToResource(newACL, newMembers)
}

// DeleteResource handles the deletion of a SCIM group resource.
func (g groupHandler) DeleteResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) error {
	resourceID := req.GetTarget().GetResourceId()

	acl, err := g.AccessListGetter.GetAccessList(ctx, resourceID)
	if err != nil {
		return trace.Wrap(err, "fetching ACL for provisioning")
	}

	if !accessListPredicate(acl) {
		return trace.NotFound("access list %q not found", resourceID)
	}

	// SCIM group deletion is not directly supported.
	// This clears members while retaining the ACL.
	if _, _, err := g.AccessListsService.UpsertAccessListWithMembers(ctx, acl, nil); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
