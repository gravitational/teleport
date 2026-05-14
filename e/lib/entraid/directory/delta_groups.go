package directory

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

type groupDeltaProcessor struct {
	// matcher is a group filter matcher.
	matcher              func(g *models.Group) bool
	entraGroupsMap       groupsByID
	entraGroupMembersMap membersByGroupID
	log                  *slog.Logger
}

// groupsByID is a Entra group map with Entra
// group ID as the map key.
type groupsByID map[entraUniqueID]*models.Group

func newGroupsDeltaProcessor(
	ctx context.Context,
	matcher func(g *models.Group) bool,
	teleportAccessListsWithMembersMap map[string]*accessListWithMembers,
	teleportUsersMap map[string]types.User,
	log *slog.Logger,
) *groupDeltaProcessor {

	// In delta sync, existing Entra ID resources previously synced to Teleport
	// is taken as the base resource collection to which the delta changes
	// are applied.
	baseBuilder := &groupBaseBuilder{
		accessLists: teleportAccessListsWithMembersMap,
		users:       maps.Clone(teleportUsersMap),
		log:         log,
	}
	groupBase := baseBuilder.build(ctx)

	return &groupDeltaProcessor{
		matcher:              matcher,
		entraGroupsMap:       groupBase.groupsMap,
		entraGroupMembersMap: groupBase.groupMembersMap,
		log:                  log,
	}
}

// apply processes new, updated or deleted group deltas
// and applies the changes to the group base created
// from Entra ID Access List.
func (g *groupDeltaProcessor) apply(ctx context.Context, in *models.ListGroupsDeltaResponse) error {
	// Don't check display name here as deleted groups delta only have ID's.
	if in == nil || in.GetID() == nil {
		return nil
	}
	groupID := entraUniqueID(*in.GetID())

	if isRemoved(in.Removed) {
		_, ok := g.entraGroupsMap[groupID]
		if !ok {
			// Delta API may replay previously deleted
			// group which no longer exists in Teleport too.
			g.log.DebugContext(ctx, `Existing Access List not found for deleted Entra ID group, `+
				`deletion of the Access List will be skipped`, "entra_group_id", groupID)
			return nil
		}

		g.remove(groupID)
		return nil
	}

	if err := validateGroup(in.Group); err != nil {
		return trace.Wrap(err)
	}

	if !g.matcher(in.Group) {
		if _, ok := g.entraGroupsMap[groupID]; ok {
			// The name of the group was updated, which no longer
			// matches with the existing group filter.
			g.remove(groupID)
			g.log.DebugContext(ctx,
				"Group removed because the updated name no longer matches with the group filter",
				"entra_group_id", groupID,
				"entra_group_name", *in.DisplayName,
				"sync_mode", mdmsync.SyncModePartial,
			)
		}
		return nil
	}

	g.put(groupID, in.Group)
	g.applyMembers(in)

	return nil
}

// put adds group to the group map.
func (g *groupDeltaProcessor) put(groupID entraUniqueID, group *models.Group) {
	g.entraGroupsMap[groupID] = group
}

// remove removes the group from group and group members map.
func (g *groupDeltaProcessor) remove(groupID entraUniqueID) {
	delete(g.entraGroupsMap, groupID)
	delete(g.entraGroupMembersMap, groupID)
}

// applyMembers processes new, updated or deleted groups
// and applies the changes to the group base created
// from the Entra ID Access List members.
func (g *groupDeltaProcessor) applyMembers(groupDelta *models.ListGroupsDeltaResponse) {
	groupID := entraUniqueID(*groupDelta.GetID())
	for _, m := range groupDelta.Members {
		if m.GetID() == nil {
			continue
		}
		memberID := entraUniqueID(*m.GetID())
		memberIn, ok := memberFromDelta(m)
		if !ok {
			continue
		}

		_, ok = g.entraGroupMembersMap[groupID]
		if !ok {
			// First time seeing this group member.

			if isRemoved(m.Removed) {
				// Nothing to do if this new members is being removed
				// because it was never added.
				continue
			}
			g.entraGroupMembersMap.put(groupID, memberID, memberIn)
			continue
		}
		// There is existing group memberships for this group.

		if isRemoved(m.Removed) {
			g.entraGroupMembersMap.remove(groupID, memberID)
			continue
		}

		// Add new member.
		g.entraGroupMembersMap.put(groupID, memberID, memberIn)
	}
}

func memberFromDelta(in models.MembersDelta) (models.GroupMember, bool) {
	// Delta responses only contains member ID, type and removed delta.
	var out models.GroupMember
	switch in.Type {
	case models.ODataUser:
		out = &models.User{
			DirectoryObject: models.DirectoryObject{
				ID: in.GetID(),
			},
		}
	case models.ODataGroup:
		out = &models.Group{
			DirectoryObject: models.DirectoryObject{
				ID: in.GetID(),
			},
		}
	default:
		// Unsupported members skipped.
		// Logging is avoided because the list can be
		// very large. Unsupported members are well documented
		// in Teleport public docs.
		return nil, false
	}
	return out, true
}

// result returns the final state of the Entra ID groups and members
// after applying delta changes to an existing group base created
// from the Entra ID Access List and members.
func (g *groupDeltaProcessor) result() listEntraGroupsResponse {
	out := listEntraGroupsResponse{
		groupsMap:       make(map[string]*models.Group),
		groupMembersMap: make(map[string][]models.GroupMember),
	}

	// TODO(sshah): use named maps in full sync response types
	// so that the conversions aren't necessary.
	for id, group := range g.entraGroupsMap {
		groupID := string(id)
		out.groupsMap[groupID] = group
	}
	for id, memberMap := range g.entraGroupMembersMap {
		groupID := string(id)
		out.groupMembersMap[groupID] = append(out.groupMembersMap[groupID],
			slices.Collect(maps.Values(memberMap))...,
		)
	}

	return out
}

// membersByID is a Entra group member map with Entra
// group member ID as the map key.
type membersByID map[entraUniqueID]models.GroupMember

// membersByGroupID is a nested group member map where
// the parent map key is the Entra group ID and nested
// map key is the member ID.
type membersByGroupID map[entraUniqueID]membersByID

// put adds group member to the members map.
func (m membersByGroupID) put(groupID, memberID entraUniqueID, memberIn models.GroupMember) {
	groupMemberMap, ok := m[groupID]
	if !ok {
		newGM := make(membersByID)
		newGM[memberID] = memberIn
		m[groupID] = newGM
		return
	}

	groupMemberMap[memberID] = memberIn
	m[groupID] = groupMemberMap
}

// remove removes group member from the members map.
func (m membersByGroupID) remove(groupID, memberID entraUniqueID) {
	groupMemberMap, ok := m[groupID]
	if !ok {
		return
	}
	delete(groupMemberMap, memberID)
	if len(groupMemberMap) == 0 {
		delete(m, groupID)
	}
}

type groupBase struct {
	groupsMap       groupsByID
	groupMembersMap membersByGroupID
}

type groupBaseBuilder struct {
	// Teleport Access List with member map.
	accessLists map[string]*accessListWithMembers
	// Teleport users map.
	users map[string]types.User
	log   *slog.Logger
}

// build builds a baseline Entra ID group,
// group member map from Teleport Access List.
func (b *groupBaseBuilder) build(ctx context.Context) groupBase {
	out := groupBase{
		groupsMap:       make(groupsByID),
		groupMembersMap: make(membersByGroupID),
	}

	for _, al := range b.accessLists {
		id, ok := al.AccessList.GetLabel(types.EntraUniqueIDLabel)
		if !ok || id == "" {
			b.log.DebugContext(ctx, "Entra unique ID label not found for Entra ID Access List",
				"access_list_name", al.AccessList.GetName(),
				"access_list_title", al.AccessList.Spec.Title,
			)
			continue
		}
		groupID := entraUniqueID(id)
		membersMap := make(membersByID)
		for _, alMember := range al.Members {
			memberID, member, ok := b.buildMember(ctx, alMember)
			if !ok {
				continue
			}
			membersMap[memberID] = member
		}
		out.groupsMap[groupID] = newGroupFromAccessList(id, al.AccessList)
		out.groupMembersMap[groupID] = membersMap
	}

	return out
}

func (b *groupBaseBuilder) buildMember(ctx context.Context, in *accesslist.AccessListMember) (entraUniqueID, models.GroupMember, bool) {
	switch in.Spec.MembershipKind {
	case accesslist.MembershipKindList:
		acl, ok := b.accessLists[in.GetName()]
		if !ok {
			b.log.DebugContext(ctx,
				"Teleport Access List not found for Entra ID Access List member",
				"access_list_member", in.GetName(),
			)
			return "", nil, false
		}
		memberID, ok := acl.GetLabel(types.EntraUniqueIDLabel)
		if !ok || memberID == "" {
			return "", nil, false
		}
		gm := &models.Group{
			DirectoryObject: models.DirectoryObject{
				ID: to.Ptr(memberID),
			},
		}
		return entraUniqueID(memberID), gm, true
	case accesslist.MembershipKindUser:
		user, ok := b.users[in.GetName()]
		if !ok {
			b.log.DebugContext(ctx, "Teleport user account not found for Entra ID Access List member",
				"access_list_member", in.GetName(),
			)
			return "", nil, false
		}
		memberID, ok := user.GetLabel(types.EntraUniqueIDLabel)
		if !ok || memberID == "" {
			return "", nil, false
		}
		gm := &models.User{
			DirectoryObject: models.DirectoryObject{
				ID: to.Ptr(memberID),
			},
		}
		return entraUniqueID(memberID), gm, true
	default:
		// Only user and nested Access List expected
		return "", nil, false
	}
}

func newGroupFromAccessList(id string, accessList *accesslist.AccessList) *models.Group {
	out := &models.Group{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr(id),
			DisplayName: &accessList.Spec.Title,
		},
		// TODO(sshah): preserve ownership when owners source is entra id.
	}
	if samAccountName, ok := accessList.GetLabel(onPremisesSamAccountNameLabel); ok && samAccountName != "" {
		out.OnPremisesSamAccountName = &samAccountName
	}
	if domainName, ok := accessList.GetLabel(onPremisesDomainNameLabel); ok && domainName != "" {
		out.OnPremisesDomainName = &domainName
	}
	if netbiosName, ok := accessList.GetLabel(onPremisesNetBiosNameLabel); ok && netbiosName != "" {
		out.OnPremisesNetBiosName = &netbiosName
	}

	return out
}
