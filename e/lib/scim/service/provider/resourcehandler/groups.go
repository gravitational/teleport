package resourcehandler

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/trait"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/lister"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	logFieldGroupId = "group_id"
)

type ProviderGroup interface {
	AccessListPredicate(context.Context, *accesslist.AccessList) bool
	UserPredicate(context.Context, types.User) bool
	OnCreatingAccessList(context.Context, *accesslist.AccessList) error
	OnCreatingAccessListMember(context.Context, *accesslist.AccessListMember) error
	GetResourceLabels() map[string]string
}

type GroupHandler struct {
	common.Config
	ProviderGroup
}

// CreateResource handles the "create group" request from the SCIM client. If an
// appropriate AccessList already exists (e.g. if to was created by the Okta
// Sync Service prior to enabling SCIM provisioning), this method will link the
// existing ACL supplied group.
//
// If no such AccessList exists, this method will create the access list, along
// with a default roles grant and other prerequisite resources.
func (h *GroupHandler) CreateResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	// We should not trust any ID given to us from the client, as it can be
	// crafted by an attacker to overwrite existing Access Lists and Roles.
	if req.GetResource().GetId() != "" {
		// Offering us an ID to use when creating new Access Lists is actually
		// suspicious enough behavior to reject the request outright.
		return nil, trace.BadParameter("ID must not be set in creation request")
	}

	newACL, newMembers, err := h.resourceToAccessList(req.GetResource())
	if err != nil {
		return nil, trace.Wrap(err, "parsing group resource")
	}

	acl, err := h.getOrCreateAccessList(ctx, newACL)
	if err != nil {
		return nil, trace.Wrap(err, "creating ACL record")
	}

	for _, m := range newMembers {
		m.Spec.AccessList = acl.GetName()
	}

	newMembers, err = h.validateMemberList(ctx, acl, newMembers)
	if err != nil {
		return nil, trace.Wrap(err, "validating member list")
	}

	finalACL, finalMembers, err := h.AccessListsService.UpsertAccessListWithMembers(ctx, acl, newMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource, err := conv.AccessListToResource(finalACL, finalMembers)
	if err != nil {
		return nil, trace.Wrap(err, "formatting response")
	}

	return resource, nil
}

func (h *GroupHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	l := lister.GroupLister{
		Config: h.Config,
		Predicate: func(accessList *accesslist.AccessList) bool {
			return h.AccessListPredicate(ctx, accessList)
		},
		AccessListToResource: func(list *accesslist.AccessList) (*scimpb.Resource, error) {
			r, err := conv.AccessListToResource(list, nil)
			return r, trace.Wrap(err)
		},
	}
	out, err := l.ListResources(ctx, req)
	return out, trace.Wrap(err)
}

func (h *GroupHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	accessList, members, err := h.loadAccessListWithMembers(ctx, req.GetTarget().GetResourceId())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource, err := conv.AccessListToResource(accessList, members)
	if err != nil {
		return nil, trace.Wrap(err, "formatting response")
	}

	return resource, nil
}

func (h *GroupHandler) UpdateResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	oldACL, oldMembers, err := h.loadAccessListWithMembers(ctx, req.GetResource().GetId())
	if err != nil {
		return nil, trace.Wrap(err, "loading existing access list")
	}

	newACL, newMembers, err := h.resourceToAccessList(req.GetResource())
	if err != nil {
		return nil, trace.Wrap(err, "parsing new access list")
	}

	newMembers, err = h.validateMemberList(ctx, newACL, newMembers)
	if err != nil {
		return nil, trace.Wrap(err, "validating new access list")
	}

	// The only access-list level thing that the SCIM resource has the data to
	// change here is the display name, so lets just update the old ACL with
	// that, rather than try to make the new ACL match the old one
	oldACL.Spec.Title = newACL.Spec.Title

	oldMembersMap := utils.FromSlice(oldMembers, oktacommon.MemberKey)
	oktaMemberMap := utils.FromSlice(newMembers, oktacommon.MemberKey)

	// Exclude Okta members who were assigned via an ongoing Access Request.
	// These temporary assignments should not be treated as long-term membership.
	f := oktacommon.OngoingAssignmentsMembershipFilter{AssignmentsService: h.AssignmentService}

	filteredOktaMembersMap, _, err := f.Filter(ctx, oktaMemberMap, oldMembersMap)
	if err != nil {
		return nil, trace.Wrap(err, "filtering members with an ongoing Access Request")
	}
	var filteredMembers []*accesslist.AccessListMember
	for _, m := range newMembers {
		if _, ok := filteredOktaMembersMap[oktacommon.MemberKey(m)]; ok {
			filteredMembers = append(filteredMembers, m)
		}
	}

	finalACL, finalMembers, err := h.AccessListsService.UpsertAccessListWithMembers(ctx, oldACL, filteredMembers)
	if err != nil {
		return nil, trace.Wrap(err, "upserting access list")
	}

	resource, err := conv.AccessListToResource(finalACL, finalMembers)
	if err != nil {
		return nil, trace.Wrap(err, "formatting response")
	}

	return resource, nil
}

func (h *GroupHandler) DeleteResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) error {
	id := req.GetTarget().GetResourceId()
	logger := h.Logger.With(logFieldGroupId, id)

	// Check that the target Access List exists and belongs to this provider
	logger.DebugContext(ctx, "Checking Access List existence and provenance")
	acl, err := h.loadAccessList(ctx, id)
	if err != nil {
		return trace.Wrap(err, "loading existing access list")
	}

	logger.DebugContext(ctx, "Deleting AccessList")
	if err := h.AccessListsService.DeleteAccessList(ctx, id); err != nil {
		return trace.Wrap(err)
	}

	// Delete the owner- and member-granted roles associated with the ACL. Teleport
	// prevents users from modifying Okta-derived Access Lists to add other
	// roles, so it should be safe to delete these. They can't be anything other
	// than the roles that were created along with the Access List itself.
	//
	// WARNING: This is a reasonable assumption while Okta is the only IdP using
	//          this SCIM service - it may need revisiting when we add more IdPs
	//          that may have different ACL modification rules.
	//
	roles := append(acl.Spec.Grants.Roles, acl.Spec.OwnerGrants.Roles...)
	for _, roleName := range roles {
		// make a best-effort attempt to delete the associated roles. Okta sync
		// will clean up any leftovers on its next synchronization pass
		if err := h.RolesService.DeleteRole(ctx, roleName); err != nil {
			logger.ErrorContext(ctx, "Access List Role deletion failed",
				"role_name", roleName,
				"error", err,
			)
		}
	}

	return nil
}

// resourceToAccessList constructs an un-validated, in-memory Teleport access
// list from the supplied SCIM resource. Note that the AccessList and
// AccessListMembers may not be fully filled-out and valid resources ready for
// presentation to the AccessList service - for example, the returned AccessList
// may not have a valid ID yet when the SCIM client is creating a new group.
//
// The IdP shim `onCreatingXXXX` callbacks give IdPs an opportunity to fill out
// the missing details before the resources are presented to the AccessList
// service
func (h *GroupHandler) resourceToAccessList(r *scimpb.Resource) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	list, members, err := conv.AccessListFromResource(r,
		conv.WithClock(h.Clock),
		conv.WithAccessListLabels(h.GetResourceLabels()),
		conv.WithGrants(accesslist.Grants{
			Roles:  []string{r.GetId()},
			Traits: trait.Traits{}},
		),
	)
	return list, members, trace.Wrap(err)
}

// createNewAccessList creates a new AccessList amd adds it to the cluster backend.
func (h *GroupHandler) createNewAccessList(ctx context.Context, acl *accesslist.AccessList) (*accesslist.AccessList, error) {
	if err := h.OnCreatingAccessList(ctx, acl); err != nil {
		return nil, trace.Wrap(err)
	}

	accessRole, reviewerRole, err := h.createACLRoles(ctx, acl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	acl.Spec.OwnerGrants.Roles = []string{reviewerRole.GetName()}
	acl.Spec.Grants.Roles = []string{accessRole.GetName()}

	h.Logger.DebugContext(ctx, "Upserting access list", "access_list", acl.GetName())
	upsertedACL, err := h.AccessListsService.UpsertAccessList(ctx, acl)
	if err != nil {
		return nil, trace.Wrap(err, "creating accesslist")
	}

	return upsertedACL, nil
}

func (h *GroupHandler) createACLRoles(ctx context.Context, acl *accesslist.AccessList) (types.Role, types.Role, error) {
	accessRoleName := oktacommon.CreateOktaAccessRoleFriendlyName(acl.Spec.Title, acl.GetName())
	reviewerRoleName := oktacommon.CreateOktaReviewerRoleFriendlyName(acl.Spec.Title, acl.GetName())

	accessRole, err := types.NewRole(accessRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindUserGroup, services.RO()),
			},
			GroupLabels: types.Labels{
				eteleport.OktaGroupIDLabel: []string{acl.GetName()},
			},
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	accessRole.SetStaticLabels(h.GetResourceLabels())

	reviewerRole, err := types.NewRole(reviewerRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{accessRole.GetName()},
			},
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	labelsCpy := h.GetResourceLabels()
	labelsCpy[eteleport.OktaACLReviewerRoleLabel] = "true"
	reviewerRole.SetStaticLabels(labelsCpy)

	h.Logger.DebugContext(ctx, "Creating access role", "role", accessRole.GetName())
	if _, err := h.RolesService.CreateRole(ctx, accessRole); err != nil {
		return nil, nil, trace.Wrap(err, "creating access role %q", accessRole.GetName())
	}

	h.Logger.DebugContext(ctx, "Creating reviewer role", "role", reviewerRole.GetName())
	if _, err := h.RolesService.CreateRole(ctx, reviewerRole); err != nil {
		return nil, nil, trace.Wrap(err, "creating reviewer role %q", reviewerRole.GetName())
	}

	return accessRole, reviewerRole, nil
}

func (h *GroupHandler) validateMemberList(ctx context.Context, acl *accesslist.AccessList, members []*accesslist.AccessListMember) ([]*accesslist.AccessListMember, error) {
	log := h.Logger.With(logFieldGroupId, acl.GetName())

	validatedUsers := make([]*accesslist.AccessListMember, 0, len(members))
	for _, m := range members {
		memberLogger := log.With("member", m.Spec.Name)

		memberLogger.DebugContext(ctx, "Processing Group Member")

		user, err := h.UsersService.GetUser(ctx, m.Spec.Name, false)
		if err != nil {
			memberLogger.ErrorContext(ctx, "Failed fetching user", "error", err)
			continue
		}

		if !h.UserPredicate(ctx, user) {
			memberLogger.DebugContext(ctx, "User does not belong to IdP")
			continue
		}

		// Give the IdP an opportunity to customize the AccessListMember record
		if err := h.OnCreatingAccessListMember(ctx, m); err != nil {
			memberLogger.ErrorContext(ctx, "Omitting member after failing to customize member record", "error", err)
			continue
		}

		// The AccessListMember record should be a valid, fully-fledged
		// AccessListMember by this point, so we should assert that this is the
		// case.
		if err := m.CheckAndSetDefaults(); err != nil {
			memberLogger.ErrorContext(ctx, "Omitting member after failing to validate record", "error", err)
			continue
		}

		validatedUsers = append(validatedUsers, m)
	}

	return validatedUsers, nil
}

// getOrCreateAccessList attempts to match a new Group with an existing
// AccessList, creating a new AccessList if no such candidate exists.
//
// Sometimes an AccessList has already been created by other means (e.g. Okta
// sync service) before it gets provisioned via SCIM, and in that case we want
// the SCIM resource to adopt to the already-existing AccessList rather than
// create a new one, and getOrCreateAccessList implements this "adoption"
// process.
func (h *GroupHandler) getOrCreateAccessList(ctx context.Context, acl *accesslist.AccessList) (*accesslist.AccessList, error) {
	oldACL, err := h.findAccessListByDisplayName(ctx, acl.Spec.Title)
	if trace.IsNotFound(err) {
		newACL, err := h.createNewAccessList(ctx, acl)
		if err != nil {
			return nil, trace.Wrap(err, "creating new access list")
		}
		return newACL, nil
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return oldACL, nil
}

func (h *GroupHandler) findAccessListByDisplayName(ctx context.Context, displayName string) (*accesslist.AccessList, error) {
	h.Logger.DebugContext(ctx, "Looking for ACL with display name", "display_name", displayName)

	var candidate *accesslist.AccessList
	var nextToken string
	var page []*accesslist.AccessList
	var err error

	for {
		page, nextToken, err = h.AccessListsService.ListAccessLists(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err, "enumerating access lists")
		}

		for _, acl := range page {
			h.Logger.DebugContext(ctx, "Examining ACL",
				slog.Group("acl",
					"name", acl.GetName(),
					"title", acl.Spec.Title),
			)

			if acl.Spec.Title != displayName {
				h.Logger.DebugContext(ctx, "Title mismatch", "existing_title", acl.Spec.Title, "requested_title", displayName)
				continue
			}

			if !h.AccessListPredicate(ctx, acl) {
				h.Logger.DebugContext(ctx, "Failed access list predicate", "labels", acl.GetMetadata().Labels)
				continue
			}

			if candidate != nil {
				return nil, trace.AlreadyExists("multiple candidates exist for AccessList %q", displayName)
			}

			candidate = acl
		}

		if nextToken == "" {
			break
		}
	}

	if candidate == nil {
		return nil, trace.NotFound("no valid candidate found")
	}

	return candidate, nil
}

// loadAccessListWithMembers fetches an AccessList and its associated member
// list. An AccessList not "owned" by the supplied shim will be considered
// "not found".
func (h *GroupHandler) loadAccessListWithMembers(ctx context.Context, id string) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	acl, err := h.loadAccessList(ctx, id)
	if err != nil {
		return nil, nil, trace.Wrap(err, "loading access list %q", id)
	}

	members, err := h.loadAccessListMembers(ctx, acl)
	if err != nil {
		return nil, nil, trace.Wrap(err, "loading access list members")
	}

	return acl, members, nil
}

func (h *GroupHandler) loadAccessList(ctx context.Context, id string) (*accesslist.AccessList, error) {
	acl, err := h.AccessListGetter.GetAccessList(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err, "loading access list")
	}

	if !h.AccessListPredicate(ctx, acl) {
		return nil, trace.NotFound("%s", id)
	}

	return acl, nil
}

func (h *GroupHandler) loadAccessListMembers(ctx context.Context, acl *accesslist.AccessList) ([]*accesslist.AccessListMember, error) {
	return h.loadAccessListMembersRecurse(ctx, acl.GetName(), 0)
}

func (h *GroupHandler) loadAccessListMembersRecurse(ctx context.Context, accessListName string, depth int32) ([]*accesslist.AccessListMember, error) {
	if depth > accesslist.MaxAllowedDepth {
		return nil, nil
	}

	nextPage := ""
	var err error
	var members []*accesslist.AccessListMember

	for {
		var page []*accesslist.AccessListMember
		page, nextPage, err = h.AccessListGetter.ListAccessListMembers(ctx, accessListName, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err, "enumerating access list members")
		}

		for _, member := range page {
			// recursively fetch members if the member is of type list
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				nestedList, err := h.AccessListGetter.GetAccessList(ctx, member.GetName())
				if err != nil {
					return nil, trace.Wrap(err, "loading nested list")
				}
				nestedMembers, err := h.loadAccessListMembersRecurse(ctx, nestedList.GetName(), depth+1)
				if err != nil {
					return nil, trace.Wrap(err, "loading members of nested list")
				}
				// don't append nil slices if depth is exceeded
				if nestedMembers != nil {
					members = append(members, nestedMembers...)
				}
			} else {
				members = append(members, member)
			}
		}

		if nextPage == "" {
			break
		}
	}

	return members, nil
}
