package scim

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/mapstructure"
	"github.com/scim2/filter-parser/v2"
	"google.golang.org/protobuf/types/known/structpb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/e/lib/okta/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

const (
	groupNameAttribute = "groupName"
	logFieldGroupId    = "group_id"
	addedBySCIM        = "scim"
	lockTTL            = 5 * time.Second
)

type groupHandler struct {
	accessLists AccessListsService
	roles       RolesService
	users       UsersService
	clock       clockwork.Clock
	logger      *slog.Logger
}

// static assertion that groupHandler implements the resourceHandler interface
var _ resourceHandler = (*groupHandler)(nil)

// create handles the "create group" request from the SCIM client. If an
// appropriate AccessList already exists (e.g. if to was created by the Okta
// Sync Service prior to enabling SCIM provisioning), this method will link the
// existing ACL supplied group.
//
// If no such AccessList exists, this method will create the access list, along
// with a default roles grant and other prerequisite resources.
func (gh *groupHandler) create(ctx context.Context, shim providerShim, r *scimpb.Resource) (*scimpb.Resource, error) {
	// We should not trust any ID given to us from the client, as it can be
	// crafted by an attacker to overwrite existing Access Lists and Roles.
	if r.Id != "" {
		// Offering us an ID to use when creating a new Access Lists is actually
		// suspicious enough behavior to reject the request outright.
		return nil, trace.BadParameter("ID must not be set in creation request")
	}

	newACL, newMembers, err := gh.resourceToAccessList(r, shim)
	if err != nil {
		return nil, trace.Wrap(err, "parsing group resource")
	}

	acl, err := gh.getOrCreateAccessList(ctx, shim, newACL)
	if err != nil {
		return nil, trace.Wrap(err, "creating ACL record")
	}

	for _, m := range newMembers {
		m.Spec.AccessList = acl.GetName()
	}

	newMembers, err = gh.validateMemberList(ctx, shim, acl, newMembers)
	if err != nil {
		return nil, trace.Wrap(err, "validating member list")
	}

	finalACL, finalMembers, err := gh.accessLists.UpsertAccessListWithMembers(ctx, acl, newMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource, err := accessListToResource(finalACL, finalMembers)
	if err != nil {
		return nil, trace.Wrap(err, "formatting response")
	}

	return resource, nil
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
func (gh *groupHandler) resourceToAccessList(r *scimpb.Resource, shim providerShim) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	group, err := decodeGroupResource(r.Attributes.AsMap())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var roles []string
	if r.Id != "" {
		roles = []string{r.Id}
	}

	labels := shim.getResourceLabels()

	// We may not have enough information to create a fully valid AccessList
	// that would be accepted by the AccessList service here - especially if
	// the SCIM group we're decoding was presented to us in order to create a
	// new group - hence we have to create the structure manually rather than
	// via accesslist.NewAccessList().
	acl := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{
				Name:   r.Id,
				Labels: labels,
			},
		},
		Spec: accesslist.Spec{
			Title: group.DisplayName,
			Grants: accesslist.Grants{
				Roles:  roles,
				Traits: trait.Traits{},
			},
		},
	}

	members := make([]*accesslist.AccessListMember, len(group.Members))
	for i, m := range group.Members {
		// We don't have enough data to make a valid AccessListMember at this
		// point, hence creating it directly rather than using the provided
		// NewAccessList() constructor, which would fail.
		newMember := &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name:   m.Value,
					Labels: labels,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList: acl.GetName(),
				Name:       m.Value,
				Joined:     gh.clock.Now(),
				AddedBy:    addedBySCIM,
			},
		}
		members[i] = newMember
	}
	return acl, members, nil
}

// createNewAccessList creates a new AccessList amd adds it to the cluster backend.
func (gh *groupHandler) createNewAccessList(ctx context.Context, shim providerShim, acl *accesslist.AccessList) (*accesslist.AccessList, error) {

	if err := shim.onCreatingAccessList(ctx, acl); err != nil {
		return nil, trace.Wrap(err)
	}

	accessRole, reviewerRole, err := gh.createACLRoles(ctx, shim, acl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	acl.Spec.OwnerGrants.Roles = []string{reviewerRole.GetName()}
	acl.Spec.Grants.Roles = []string{accessRole.GetName()}

	gh.logger.DebugContext(ctx, "Upserting access list", "access_list", acl.GetName())
	upsertedACL, err := gh.accessLists.UpsertAccessList(ctx, acl)
	if err != nil {
		return nil, trace.Wrap(err, "creating accesslist")
	}

	return upsertedACL, nil
}

func (gh *groupHandler) createACLRoles(ctx context.Context, shim providerShim, acl *accesslist.AccessList) (types.Role, types.Role, error) {
	accessRoleName := common.CreateOktaAccessRoleFriendlyName(acl.Spec.Title, acl.GetName())
	reviewerRoleName := common.CreateOktaReviewerRoleFriendlyName(acl.Spec.Title, acl.GetName())

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
	accessRole.SetStaticLabels(shim.getResourceLabels())

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
	labelsCpy := shim.getResourceLabels()
	labelsCpy[eteleport.OktaACLReviewerRoleLabel] = "true"
	reviewerRole.SetStaticLabels(labelsCpy)

	gh.logger.DebugContext(ctx, "Creating access role", "role", accessRole.GetName())
	if _, err := gh.roles.CreateRole(ctx, accessRole); err != nil {
		return nil, nil, trace.Wrap(err, "creating access role %q", accessRole.GetName())
	}

	gh.logger.DebugContext(ctx, "Creating reviewer role", "role", reviewerRole.GetName())
	if _, err := gh.roles.CreateRole(ctx, reviewerRole); err != nil {
		return nil, nil, trace.Wrap(err, "creating reviewer role %q", reviewerRole.GetName())
	}

	return accessRole, reviewerRole, nil
}

func (gh *groupHandler) validateMemberList(ctx context.Context, shim providerShim, acl *accesslist.AccessList, members []*accesslist.AccessListMember) ([]*accesslist.AccessListMember, error) {
	log := gh.logger.With(logFieldGroupId, acl.GetName())

	validatedUsers := make([]*accesslist.AccessListMember, 0, len(members))
	for _, m := range members {
		memberLogger := log.With("member", m.Spec.Name)

		memberLogger.DebugContext(ctx, "Processing Group Member")

		user, err := gh.users.GetUser(ctx, m.Spec.Name, false)
		if err != nil {
			memberLogger.ErrorContext(ctx, "Failed fetching user", "error", err)
			continue
		}

		if !shim.userPredicate(ctx, user) {
			memberLogger.DebugContext(ctx, "User does not belong to IdP")
			continue
		}

		// Give the IdP an opportunity to customize the AccessListMember record
		if err := shim.onCreatingAccessListMember(ctx, m); err != nil {
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

// list handles a bulk listing query from the client
func (gh *groupHandler) list(ctx context.Context, shim providerShim, filter filter.Expression, requestedPage *scimpb.Page) (*scimpb.ResourceList, error) {
	const pageSize = 100
	var outputResources []*scimpb.Resource
	index := 0
	totalCount := 0
	nextToken := ""

	for {
		var srcPage []*accesslist.AccessList
		var err error
		srcPage, nextToken, err = gh.accessLists.ListAccessLists(ctx, pageSize, nextToken)
		if err != nil {
			return nil, trace.Wrap(err, "failed enumerating AccessLists")
		}

		for _, accessList := range srcPage {
			if !shim.accessListPredicate(ctx, accessList) {
				continue
			}

			filterAttribs := map[string]string{groupNameAttribute: accessList.GetName()}
			if err := evaluateFilter(filter, filterAttribs); err != nil {
				continue
			}

			index++
			if index < int(requestedPage.StartIndex) {
				continue
			}

			if len(outputResources) < int(requestedPage.Count) {
				groupResource, err := accessListToResource(accessList, nil)
				if err != nil {
					gh.logger.ErrorContext(ctx, "converting access list to SCIM group resource",
						"error", err,
						logFieldGroupId, accessList.GetName(),
					)
					continue
				}

				outputResources = append(outputResources, groupResource)
			}

			totalCount++
		}

		if nextToken == "" {
			break
		}
	}

	output := &scimpb.ResourceList{
		TotalResults: int32(totalCount),
		StartIndex:   int32(requestedPage.StartIndex),
		ItemsPerPage: int32(requestedPage.Count),
		Resources:    outputResources,
	}

	return output, nil
}

// getOrCreateAccessList attempts to match a new Group with an existing
// AccessList, creating a new AccessList if no such candidate exists.
//
// Sometimes an AccessList has already been created by other means (e.g. Okta
// sync service) before it gets provisioned via SCIM, and in that case we want
// the SCIM resource to adopt to the already-existing AccessList rather than
// create a new one, and getOrCreateAccessList implements this "adoption"
// process.
func (gh *groupHandler) getOrCreateAccessList(ctx context.Context, shim providerShim, acl *accesslist.AccessList) (*accesslist.AccessList, error) {
	oldACL, err := gh.findAccessListByDisplayName(ctx, shim, acl.Spec.Title)
	if trace.IsNotFound(err) {
		newACL, err := gh.createNewAccessList(ctx, shim, acl)
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

func (gh *groupHandler) findAccessListByDisplayName(ctx context.Context, shim providerShim, displayName string) (*accesslist.AccessList, error) {
	gh.logger.DebugContext(ctx, "Looking for ACL with display name", "display_name", displayName)

	var candidate *accesslist.AccessList
	var nextToken string
	var page []*accesslist.AccessList
	var err error

	for {
		page, nextToken, err = gh.accessLists.ListAccessLists(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err, "enumerating access lists")
		}

		for _, acl := range page {
			gh.logger.DebugContext(ctx, "Examining ACL",
				slog.Group("acl",
					"name", acl.GetName(),
					"title", acl.Spec.Title),
			)

			if acl.Spec.Title != displayName {
				gh.logger.DebugContext(ctx, "Title mismatch", "existing_title", acl.Spec.Title, "requested_title", displayName)
				continue
			}

			if !shim.accessListPredicate(ctx, acl) {
				gh.logger.DebugContext(ctx, "Failed access list predicate", "labels", acl.GetMetadata().Labels)
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
func (gh *groupHandler) loadAccessListWithMembers(ctx context.Context, shim providerShim, id string) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	acl, err := gh.loadAccessList(ctx, shim, id)
	if err != nil {
		return nil, nil, trace.Wrap(err, "loading access list %q", id)
	}

	members, err := gh.loadAccessListMembers(ctx, acl)
	if err != nil {
		return nil, nil, trace.Wrap(err, "loading access list members")
	}

	return acl, members, nil
}

func (gh *groupHandler) loadAccessList(ctx context.Context, shim providerShim, id string) (*accesslist.AccessList, error) {
	acl, err := gh.accessLists.GetAccessList(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err, "loading access list")
	}

	if !shim.accessListPredicate(ctx, acl) {
		return nil, trace.NotFound("%s", id)
	}

	return acl, nil
}

func (gh *groupHandler) loadAccessListMembers(ctx context.Context, acl *accesslist.AccessList) ([]*accesslist.AccessListMember, error) {
	return gh.loadAccessListMembersRecurse(ctx, acl.GetName(), 0)
}

func (gh *groupHandler) loadAccessListMembersRecurse(ctx context.Context, accessListName string, depth int32) ([]*accesslist.AccessListMember, error) {
	if depth > accesslist.MaxAllowedDepth {
		return nil, nil
	}

	nextPage := ""
	var err error
	members := []*accesslist.AccessListMember{}

	for {
		var page []*accesslist.AccessListMember
		page, nextPage, err = gh.accessLists.ListAccessListMembers(ctx, accessListName, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err, "enumerating access list members")
		}

		for _, member := range page {
			// recursively fetch members if the member is of type list
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				nestedList, err := gh.accessLists.GetAccessList(ctx, member.GetName())
				if err != nil {
					return nil, trace.Wrap(err, "loading nested list")
				}
				nestedMembers, err := gh.loadAccessListMembersRecurse(ctx, nestedList.GetName(), depth+1)
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

// get handles a request for a single group resource
func (gh *groupHandler) get(ctx context.Context, shim providerShim, id string) (*scimpb.Resource, error) {
	accessList, members, err := gh.loadAccessListWithMembers(ctx, shim, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource, err := accessListToResource(accessList, members)
	if err != nil {
		return nil, trace.Wrap(err, "formatting response")
	}

	return resource, nil
}

func (gh *groupHandler) update(ctx context.Context, shim providerShim, r *scimpb.Resource) (*scimpb.Resource, error) {
	oldACL, err := gh.loadAccessList(ctx, shim, r.Id)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing access list")
	}

	newACL, newMembers, err := gh.resourceToAccessList(r, shim)
	if err != nil {
		return nil, trace.Wrap(err, "parsing new access list")
	}

	newMembers, err = gh.validateMemberList(ctx, shim, newACL, newMembers)
	if err != nil {
		return nil, trace.Wrap(err, "validating new access list")
	}

	// The only access-list level thing that the SCIM resource has the data to
	// change here is the display name, so lets just update the old ACL with
	// that, rather than try to make the new ACL match the old one
	oldACL.Spec.Title = newACL.Spec.Title

	finalACL, finalMembers, err := gh.accessLists.UpsertAccessListWithMembers(ctx, oldACL, newMembers)
	if err != nil {
		return nil, trace.Wrap(err, "upserting access list")
	}

	resource, err := accessListToResource(finalACL, finalMembers)
	if err != nil {
		return nil, trace.Wrap(err, "formatting response")
	}

	return resource, nil
}

// delete handles a request to delete a group resource
func (gh *groupHandler) delete(ctx context.Context, shim providerShim, id string) error {
	logger := gh.logger.With(logFieldGroupId, id)

	// Check that the target Access List exists and belongs to this provider
	logger.DebugContext(ctx, "Checking Access List existence and provenance")
	acl, err := gh.loadAccessList(ctx, shim, id)
	if err != nil {
		return trace.Wrap(err, "loading existing access list")
	}

	logger.DebugContext(ctx, "Deleting AccessList")
	if err := gh.accessLists.DeleteAccessList(ctx, id); err != nil {
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
		if err := gh.roles.DeleteRole(ctx, roleName); err != nil {
			logger.ErrorContext(ctx, "Access List Role deletion failed",
				"role_name", roleName,
				"error", err,
			)
		}
	}

	return nil
}

// member holds a SCIM group membership record as per RFC 7643 Section 4.2
type member struct {
	Value   string `mapstructure:"value"`
	Display string `mapstructure:"display"`
}

// groupResource uses holds a parsed representation of a SCIM group resource,
// as per RFC 7643 Section 4.2
type groupResource struct {
	DisplayName string   `mapstructure:"displayName"`
	Members     []member `mapstructure:"members"`
}

// decodeGroupResource parses a SCIM group resource using `mapstructure`
func decodeGroupResource(attributes map[string]any) (groupResource, error) {
	var group groupResource
	if err := mapstructure.Decode(attributes, &group); err != nil {
		return groupResource{}, trace.Wrap(err)
	}
	return group, nil
}

// accessListToResource encodes a teleport AccessList and its associated
// AccessListMember records into an ARFC 7543-compliant SCIM group resource
func accessListToResource(accessList *accesslist.AccessList, members []*accesslist.AccessListMember) (*scimpb.Resource, error) {
	// The mapstructure package doesn't handle slices well when encoding to an
	// attribute map so we have to do it all manually.
	// See https://github.com/mitchellh/mapstructure/issues/249

	memberResources := make([]any, 0, len(members))
	for _, m := range members {
		memberResources = append(memberResources,
			map[string]any{"display": m.GetName(), "value": m.GetName()})
	}

	groupAttrs := map[string]any{
		"displayName": accessList.Spec.Title,
		"members":     memberResources,
	}

	attrs, err := structpb.NewStruct(groupAttrs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource := &scimpb.Resource{
		Id: accessList.GetName(),
		Meta: &scimpb.Meta{
			ResourceType: resourceTypeGroup,
			Version:      accessList.GetRevision(),
		},
		Attributes: attrs,
	}
	return resource, nil
}
