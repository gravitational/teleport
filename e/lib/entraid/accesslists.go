package entraid

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/utils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func (r *DirectoryReconciler) reconcileAccessLists(ctx context.Context,
	usersByEntraID map[entraUniqueID]types.User,
	groupsMap map[string]*msgraph.Group,
	groupMembersMap map[string][]msgraph.GroupMember,
) error {
	teleportAccessLists, err := listTeleportAccessLists(ctx, r.accessListSvc)
	if err != nil {
		return trace.Wrap(err)
	}
	entraAccessLists := convertEntraAccessLists(ctx, groupsMap, r.tenantID, r.defaultOwners)

	for name, dst := range entraAccessLists {
		src, ok := teleportAccessLists[name]
		if !ok {
			continue
		}
		preserveAccessListMetadata(dst, src)
	}

	alReconciler, err := services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher:             matchByLabel[*accesslist.AccessList],
		GetCurrentResources: func() map[string]*accesslist.AccessList { return teleportAccessLists },
		GetNewResources:     func() map[string]*accesslist.AccessList { return entraAccessLists },
		OnCreate: func(ctx context.Context, al *accesslist.AccessList) error {
			_, err := r.accessListSvc.UpsertAccessList(ctx, al)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessList, existing *accesslist.AccessList) error {
			_, err := r.accessListSvc.UpsertAccessList(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, al *accesslist.AccessList) error {
			err := r.accessListSvc.DeleteAccessList(ctx, al.GetName())
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessListValues := make([]*accesslist.AccessList, 0, len(teleportAccessLists))
	for _, al := range teleportAccessLists {
		accessListValues = append(accessListValues, al)
	}
	teleportMembers, err := listTeleportAccessListMembers(ctx, r.accessListSvc, accessListValues)
	if err != nil {
		return trace.Wrap(err)
	}

	entraMembers, err := convertEntraAccessListMembers(ctx, usersByEntraID, entraAccessLists, groupMembersMap)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, src := range teleportMembers {
		if dst, ok := entraMembers[src.GetName()]; ok {
			preserveAccessListMemberMetadata(dst, src)
		}
	}

	memberReconciler, err := services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessListMember]{
		Matcher:             matchByLabel[*accesslist.AccessListMember],
		GetCurrentResources: func() map[string]*accesslist.AccessListMember { return teleportMembers },
		GetNewResources:     func() map[string]*accesslist.AccessListMember { return entraMembers },
		OnCreate: func(ctx context.Context, m *accesslist.AccessListMember) error {
			_, err := r.accessListSvc.UpsertAccessListMember(ctx, m)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessListMember, existing *accesslist.AccessListMember) error {
			_, err := r.accessListSvc.UpsertAccessListMember(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, m *accesslist.AccessListMember) error {
			err := r.accessListSvc.DeleteAccessListMember(ctx, m.Spec.AccessList, m.Spec.Name)
			// As access lists are reconciled before members,
			// an access list removed from entra can get removed before we try to unassign members.
			// In such cases, simply ignore the "access list not found" error
			if trace.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = trace.NewAggregate(alReconciler.Reconcile(ctx), memberReconciler.Reconcile(ctx))
	if err != nil {
		return trace.Wrap(err)
	}

	r.importedGroups = len(entraAccessLists)
	return nil
}

func listTeleportAccessLists(ctx context.Context, svc accessListAccessPoint) (map[string]*accesslist.AccessList, error) {
	result := map[string]*accesslist.AccessList{}

	var accessLists []*accesslist.AccessList
	var pageToken string
	var err error
	for {
		accessLists, pageToken, err = svc.ListAccessLists(ctx, 0 /* use the default page size*/, pageToken)
		if err != nil {
			return nil, trace.Wrap(err, "listing teleport entra users")
		}

		for _, al := range accessLists {
			if matchByLabel(al) {
				result[al.GetName()] = al
			}
		}

		if pageToken == "" {
			break
		}
	}

	return result, nil
}

func convertEntraAccessLists(ctx context.Context, groupsMap map[string]*msgraph.Group, tenantID string, defaultOwners []accesslist.Owner) map[string]*accesslist.AccessList {
	result := map[string]*accesslist.AccessList{}
	for _, g := range groupsMap {
		al, err := convertGroup(g, tenantID, defaultOwners)
		if err == nil {
			result[al.GetName()] = al
		} else {
			slog.ErrorContext(ctx, "failed to convert Entra ID group to Teleport access list", "error", err)
		}
	}

	return result
}

func listTeleportAccessListMembers(ctx context.Context, svc accessListAccessPoint, als []*accesslist.AccessList) (map[string]*accesslist.AccessListMember, error) {
	result := map[string]*accesslist.AccessListMember{}

	var members []*accesslist.AccessListMember
	var pageToken string
	var err error
	for _, al := range als {
		for {
			members, pageToken, err = svc.ListAccessListMembers(ctx, al.GetName(), 0 /* use the default page size */, pageToken)
			if err != nil {
				return nil, trace.Wrap(err, "listing teleport access list members")
			}
			for _, alm := range members {
				if matchByLabel(alm) {
					result[memberMapKey(alm)] = alm
				}
			}
			if pageToken == "" {
				break
			}
		}
	}

	return result, nil
}

// NB: this enriches Access Lists passed in `als` with their child Access Lists.
func convertEntraAccessListMembers(ctx context.Context, entraUsersByID map[entraUniqueID]types.User,
	als map[string]*accesslist.AccessList,
	groupMembersMap map[string][]msgraph.GroupMember,
) (map[string]*accesslist.AccessListMember, error) {
	result := map[string]*accesslist.AccessListMember{}
	accesslistsById := map[entraUniqueID]*accesslist.AccessList{}

	for _, al := range als {
		id, ok := al.GetLabel(types.EntraUniqueIDLabel)
		if !ok {
			continue
		}
		accesslistsById[entraUniqueID(id)] = al
	}
	// TODO(justinas): look into batching this if possible.
	for _, al := range als {
		id, ok := al.GetLabel(types.EntraUniqueIDLabel)
		if !ok {
			return nil, trace.BadParameter("access list %v missing Entra ID unique ID label", al.GetName())
		}
		for _, member := range groupMembersMap[id] {
			alm, err := convertGroupMember(ctx, member, al, entraUsersByID, accesslistsById)
			if err != nil {
				var id string
				if member.GetID() != nil {
					id = *member.GetID()
				}
				slog.WarnContext(ctx, "error while converting group member", "member", id, "error", err)
				continue
			}
			if alm == nil {
				slog.WarnContext(ctx, "unsupported group member, skipping")
				continue
			}
			result[memberMapKey(alm)] = alm
		}
	}
	return result, nil
}

func convertGroup(in *msgraph.Group, tenantID string, defaultOwners []accesslist.Owner) (*accesslist.AccessList, error) {
	if in.DisplayName == nil {
		return nil, trace.BadParameter("expected Entra ID group to have a non-empty display name")
	}
	displayName := *in.DisplayName
	if in.ID == nil {
		return nil, trace.BadParameter("expected Entra ID group to have a non-empty ID")
	}
	id := *in.ID

	out, err := accesslist.NewAccessList(
		header.Metadata{
			Name: accessListName(displayName, id),
		},
		accesslist.Spec{
			Title:  displayName,
			Owners: defaultOwners,
			Grants: accesslist.Grants{
				Traits: trait.Traits{
					eteleport.EntraMemberOfGroupTrait: {id},
				},
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.SetStaticLabels(map[string]string{
		types.EntraTenantIDLabel:    tenantID,
		types.EntraUniqueIDLabel:    id,
		types.EntraDisplayNameLabel: displayName,
	})
	out.SetOrigin(types.OriginEntraID)
	return out, nil
}

// convertGroupMember converts an Entra group member to an AccessListMember.
// Error is returned on unexpected conditions, indicating programmer error (e.g. validation of AccessListMember fails).
// On non fatal errors, e.g. an unsupported member type, a warning is logged and (nil, nil) is returned.
func convertGroupMember(ctx context.Context,
	in msgraph.GroupMember,
	al *accesslist.AccessList,
	entraUsersByID map[entraUniqueID]types.User,
	accesslistByEntraId map[entraUniqueID]*accesslist.AccessList,
) (*accesslist.AccessListMember, error) {
	if in.GetID() == nil {
		return nil, trace.BadParameter("expected Entra ID user to have a non-empty unique ID")
	}
	id := *in.GetID()

	switch in.(type) {
	case *msgraph.User:
		teleportUser, ok := entraUsersByID[entraUniqueID(id)]
		if !ok {
			slog.WarnContext(ctx, "no teleport user found for Entra unique ID", "id", id)
			return nil, nil
		}
		alm, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: teleportUser.GetName(),
			},
			accesslist.AccessListMemberSpec{
				AccessList:     al.GetName(),
				Name:           teleportUser.GetName(),
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	case *msgraph.Group:
		accessList, ok := accesslistByEntraId[entraUniqueID(id)]
		if !ok {
			slog.WarnContext(ctx, "No access list found for Entra unique ID", "id", id)
			return nil, nil
		}
		alm, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: accessList.GetName(),
			},
			accesslist.AccessListMemberSpec{
				AccessList:     al.GetName(),
				Name:           accessList.GetName(),
				Joined:         time.Now().UTC(),
				AddedBy:        teleport.UserSystem,
				MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	default:
		slog.WarnContext(ctx, "entra group member is not of a supported type: ", "directory_object", in, "type", logutils.TypeAttr(in))
		return nil, nil
	}
}

// uuidNamespace is the namespace used for generating UUIDs for access lists.
// It is a UUID derived from the string "entraid".
var uuidNamespace = uuid.NewSHA1(uuid.Nil, []byte("entraid"))

func accessListName(displayName string, id string) string {
	p := path.Join(id, displayName)
	// generate a UUID from the path to ensure uniqueness
	// and to avoid collisions with other access lists.
	// This is necessary because access list names are used as keys in the backend
	// and must be unique and deterministic.
	return uuid.NewSHA1(uuidNamespace, []byte(p)).String()
}

func preserveAccessListMemberMetadata(dst, src *accesslist.AccessListMember) {
	dst.Spec.Joined = src.Spec.Joined
}

func memberMapKey(member *accesslist.AccessListMember) string {
	return fmt.Sprintf("%s/%s", member.Spec.AccessList, member.GetName())
}

func listEntraGroups(ctx context.Context, graphClient GraphClient) (map[string]*msgraph.Group, error) {
	result := map[string]*msgraph.Group{}
	err := graphClient.IterateGroups(ctx, func(g *msgraph.Group) bool {
		result[*g.ID] = g
		return true
	})
	return result, trace.Wrap(err)
}

func listEntraGroupsMembers(ctx context.Context, graphClient GraphClient, groups map[string]*msgraph.Group) (map[string][]msgraph.GroupMember, error) {
	result := map[string][]msgraph.GroupMember{}
	for id, group := range groups {
		var members []msgraph.GroupMember
		err := graphClient.IterateGroupMembers(ctx, *group.ID, func(member msgraph.GroupMember) bool {
			members = append(members, member)
			return true
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result[id] = members
	}
	return result, nil
}

func unwindGroupMembership(groups map[string]*msgraph.Group, groupMembers map[string][]msgraph.GroupMember) map[string][]string {
	// result map to hold the membership paths for each group.
	result := make(map[string][]string)
	// visited tracks groups in the current path to avoid cycles.
	visited := make(map[string]bool)

	var collectPaths func(groupID string) []string
	collectPaths = func(groupID string) []string {
		// if the path for this group is already computed, return it.
		if path, exists := result[groupID]; exists {
			return path
		}

		// if the group is already visited in the current path, it means there is a cycle.
		if visited[groupID] {
			return []string{groupID} // Break the cycle by not proceeding further in this branch
		}

		visited[groupID] = true

		// path always starts with the current group.
		path := []string{groupID}

		// Traverse each member of the group.
		for _, member := range groupMembers[groupID] {
			if nestedGroup, ok := member.(*msgraph.Group); ok {
				// Skip Office 365 groups, we only care about security groups.
				if nestedGroup.IsOffice365Group() {
					continue
				}
				// recursively collect the path for nested groups.
				nestedPath := collectPaths(*nestedGroup.ID)
				path = append(path, nestedPath...)
			}
		}

		// store the computed path in result to avoid redundant calculations
		result[groupID] = path

		// unmark this group as visited after recursion completes
		visited[groupID] = false
		return path
	}

	// Collect the membership paths for each group.
	for groupID, group := range groups {
		// Skip Office 365 groups, we only care about security groups.
		if group.IsOffice365Group() {
			continue
		}
		collectPaths(groupID)
	}

	// Convert the result to a map where each group ID maps to a list of group IDs
	// that a user member of that group is automatically a member of.
	groupToMembership := make(map[string][]string)
	for childGroup, v := range result {
		for _, parentGroup := range v {
			groupToMembership[parentGroup] = append(groupToMembership[parentGroup], childGroup)
		}
	}
	for k, v := range groupToMembership {
		groupToMembership[k] = utils.Deduplicate(v)
	}
	return groupToMembership
}

func preserveAccessListMetadata(dst, src *accesslist.AccessList) {
	dst.Status = src.Status
	dst.Metadata.Revision = src.Metadata.Revision
	dst.Spec.Audit = src.Spec.Audit
	dst.Spec.Description = src.Spec.Description

	dst.Spec.Grants.Roles = src.Spec.Grants.Roles
	for k, v := range src.Spec.Grants.Traits {
		dstVal, ok := dst.Spec.Grants.Traits[k]
		if !ok {
			dst.Spec.Grants.Traits[k] = v
			continue
		}
		dst.Spec.Grants.Traits[k] = utils.Deduplicate(append(dstVal, v...))
	}
}
