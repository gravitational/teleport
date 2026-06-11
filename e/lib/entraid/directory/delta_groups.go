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
	entraGroupOwnersMap  ownersByGroupID
	setEntraGroupOwners  bool
	log                  *slog.Logger
}

type groupDeltaProcessorConfig struct {
	matcher func(g *models.Group) bool
	// accessListsMap is a map of Acess List with members where
	// map key is the resource name of the Access List.
	accessListsMap map[string]*accessListWithMembers
	// teleportUsersMap is a map of Teleport users where
	// map key is the resource name of the user.
	teleportUsersMap    map[string]types.User
	setEntraGroupOwners bool
	log                 *slog.Logger
}

func newGroupsDeltaProcessor(
	ctx context.Context,
	cfg groupDeltaProcessorConfig,
) *groupDeltaProcessor {

	// In delta sync, existing Entra ID resources previously synced to Teleport
	// is taken as the base resource collection to which the delta changes
	// are applied.
	baseBuilder := &groupBaseBuilder{
		accessLists:         cfg.accessListsMap,
		users:               maps.Clone(cfg.teleportUsersMap),
		setEntraGroupOwners: cfg.setEntraGroupOwners,
		log:                 cfg.log,
	}
	groupBase := baseBuilder.build(ctx)

	return &groupDeltaProcessor{
		matcher:              cfg.matcher,
		entraGroupsMap:       groupBase.groupsMap,
		entraGroupMembersMap: groupBase.groupMembersMap,
		entraGroupOwnersMap:  groupBase.groupOwnersMap,
		setEntraGroupOwners:  cfg.setEntraGroupOwners,
		log:                  cfg.log,
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
			g.log.DebugContext(ctx, `Existing Access List not found for deleted Entra ID group, deletion of the Access List will be skipped`,
				"sync_mode", mdmsync.SyncModePartial,
				"group_id", groupID)
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
				"sync_mode", mdmsync.SyncModePartial,
				"group_id", groupID,
				"group_name", *in.DisplayName,
			)
		}
		return nil
	}

	g.put(groupID, in.Group)
	g.applyOwners(in)
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
	delete(g.entraGroupOwnersMap, groupID)
	delete(g.entraGroupMembersMap, groupID)
}

// applyMembers processes new, updated or deleted groups members
// and applies the changes to the group base created from the
// Entra ID Access List members.
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

// applyOwners processes new, updated or deleted group owners
// and applies the changes to the group base created from the
// Entra ID Access List owners.
func (g *groupDeltaProcessor) applyOwners(groupDelta *models.ListGroupsDeltaResponse) {
	if !g.setEntraGroupOwners {
		return
	}
	groupID := entraUniqueID(*groupDelta.GetID())
	for _, o := range groupDelta.Owners {
		if o.GetID() == nil {
			continue
		}
		ownerID := entraUniqueID(*o.GetID())
		ownerIn, ok := ownerFromDelta(o)
		if !ok {
			continue
		}
		if isRemoved(o.Removed) {
			g.entraGroupOwnersMap.remove(groupID, ownerID)
		} else {
			g.entraGroupOwnersMap.put(groupID, ownerID, ownerIn)
		}
	}
}

func ownerFromDelta(in models.OwnersDelta) (*models.User, bool) {
	// Delta responses only contains owner ID, type and removed delta.
	if in.Type == models.ODataUser {
		// A valid msgraph user with ID, UPN and mail is needed to
		// configure Access List owners. But only ID is available
		// in the owner delta object. UPN and mail will be configured
		// later from the Entra ID users map.
		return &models.User{
			DirectoryObject: models.DirectoryObject{
				ID: in.GetID(),
			},
		}, true
	}
	// Unsupported owners skipped.
	// Logging is avoided because the list can be
	// very large. Unsupported members are well documented
	// in Teleport public docs.
	return nil, false
}

// result returns the final state of the Entra ID groups and members
// after applying delta changes to an existing group base created
// from the Entra ID Access List and members.
func (g *groupDeltaProcessor) result() entraGroups {
	out := entraGroups{
		groupsMap:       g.entraGroupsMap,
		groupMembersMap: make(groupMembersByGroupID),
	}
	if g.setEntraGroupOwners {
		for groupID, group := range g.entraGroupsMap {
			group.Owners = slices.Collect(maps.Values(g.entraGroupOwnersMap[groupID]))
			out.groupsMap[groupID] = group
		}
	}
	for groupID, memberMap := range g.entraGroupMembersMap {
		out.groupMembersMap[groupID] = append(out.groupMembersMap[groupID],
			slices.Collect(maps.Values(memberMap))...,
		)
	}

	return out
}

// ownersByID is a Entra group owner map with Entra
// user (owner) ID as the map key.
type ownersByID map[entraUniqueID]*models.User

// ownersByGroupID is a nested group owner map where
// the parent map key is the Entra group ID and nested
// map key is the owner ID.
type ownersByGroupID map[entraUniqueID]ownersByID

// put adds group owner to the owners map.
func (o ownersByGroupID) put(groupID, ownerID entraUniqueID, ownerIn *models.User) {
	groupOwnerMap, ok := o[groupID]
	if !ok {
		// First time adding a group owner for this group.
		groupOwnerMap = make(ownersByID)
		o[groupID] = groupOwnerMap
	}

	groupOwnerMap[ownerID] = ownerIn
}

// remove removes group owner from the owners map.
func (o ownersByGroupID) remove(groupID, ownerID entraUniqueID) {
	groupOwnerMap, ok := o[groupID]
	if !ok {
		// Likely a removed owner object that was never added
		// to Teleport or is a replayed delete delta response.
		return
	}
	delete(groupOwnerMap, ownerID)
	if len(groupOwnerMap) == 0 {
		delete(o, groupID)
	}
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
	groupOwnersMap  ownersByGroupID
	groupMembersMap membersByGroupID
}

type groupBaseBuilder struct {
	// Teleport Access List with member map.
	accessLists map[string]*accessListWithMembers
	// Teleport users map.
	users               map[string]types.User
	setEntraGroupOwners bool
	log                 *slog.Logger
}

// build builds a baseline Entra ID group, group member
// and group owner map from Teleport Access List.
func (b *groupBaseBuilder) build(ctx context.Context) groupBase {
	out := groupBase{
		groupsMap:       make(groupsByID),
		groupOwnersMap:  make(ownersByGroupID),
		groupMembersMap: make(membersByGroupID),
	}

	for _, al := range b.accessLists {
		id, ok := al.AccessList.GetLabel(types.EntraUniqueIDLabel)
		if !ok || id == "" {
			b.log.DebugContext(ctx, "Entra unique ID label not found for Entra ID Access List",
				"sync_mode", mdmsync.SyncModePartial,
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
		if b.setEntraGroupOwners {
			out.groupOwnersMap[groupID] = groupOwnersFromAccessList(al.AccessList, b.users)
		}
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
				"sync_mode", mdmsync.SyncModePartial,
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

// groupOwnersFromAccessList converts Access List owners to Entra ID group owners.
//
// Note: When Entra ID is configured to be the source of the Access List owners
// but the Entra ID group has zero owners, the service falls back to using default
// owners for that group. If the next delta sync discovers an actual new group
// owner, it ideally should remove the previously added default owners.
// But the service does not store any metadata to distinguish whether the owner
// is actually a configured owner or a default owner. As a consequence of that,
// groupOwnersFromAccessList collects all the current Access List owners and does
// not filter owners used as a fallback. The issue isn't itself very concerning
// because the default owners are users set up by the admin and only used as a
// fallback in this case. And they will be reconciled properly by a next full sync.
//
// TODO(sshah): Discuss if this issue can be accepted with a limitation laid
// out in the documentation or investigate a way to identify owners used as a
// fallback vs. actual owners that can be removed when a new owner is available
// for a group.
func groupOwnersFromAccessList(accessList *accesslist.AccessList, teleportUsersMap map[string]types.User) ownersByID {
	ownerMap := make(ownersByID)
	for _, o := range accessList.Spec.Owners {
		if !o.IsMembershipKindUser() {
			continue
		}
		// Only user as acl owner is supported for entra integration.

		// If a user account is not found or is missing EntraUniqueIDLabel, it
		// could be a default owner having Teleport local user account
		// or an SSO user from a different IdP. It's ok to skip such owners
		// here as they will be re-added as owner in the later stage based
		// on the Access List owners source config.
		user, ok := teleportUsersMap[o.Name]
		if !ok {
			continue
		}
		id, ok := user.GetLabel(types.EntraUniqueIDLabel)
		if !ok || id == "" {
			continue
		}
		ownerMap[entraUniqueID(id)] = entraOwnerFromTeleportUser(user, id)
	}
	return ownerMap
}

func entraOwnerFromTeleportUser(in types.User, ownerID string) *models.User {
	entraUser := &models.User{
		DirectoryObject: models.DirectoryObject{
			ID: to.Ptr(ownerID),
		},
	}
	if upn, ok := in.GetLabel(types.EntraUPNLabel); ok && upn != "" {
		entraUser.UserPrincipalName = to.Ptr(upn)
	}
	traits := in.GetTraits()
	if mail, ok := traits[entraIDSAMLClaimEmail]; ok && len(mail) != 0 {
		entraUser.Mail = to.Ptr(mail[0])
	}
	if display, ok := traits[displayNameClaim]; ok && len(display) != 0 {
		entraUser.DisplayName = to.Ptr(display[0])
	}

	return entraUser
}
