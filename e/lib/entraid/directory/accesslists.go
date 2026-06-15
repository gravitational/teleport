package directory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"
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
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

const (
	// https://learn.microsoft.com/en-us/graph/api/resources/group?view=graph-rest-1.0#properties

	// onPremisesSamAccountNameLabel is the Entra ID onPremisesSamAccountName
	// property which represents on-premise SAM account name.
	onPremisesSamAccountNameLabel = types.TeleportInternalLabelPrefix + "on-premises-sam-account-name"
	// onPremisesDomainNameLabel is the Entra ID onPremisesDomainName
	// property which represents the on-premise dnsDomainName value.
	onPremisesDomainNameLabel = types.TeleportInternalLabelPrefix + "on-premises-domain-name"
	// onPremisesNetBiosNameLabel is the Entra ID onPremisesNetBiosName
	// property which represents the on-premise netBios name.
	onPremisesNetBiosNameLabel = types.TeleportInternalLabelPrefix + "on-premises-net-bios-name"
	// groupTypeLabel is the Entra ID group type property.
	groupTypeLabel = types.TeleportInternalLabelPrefix + "group-types"
)

// groupsByID is a Entra group map with Entra
// group ID as the map key.
type groupsByID map[entraUniqueID]*models.Group

// groupMembersByGroupID is a Entra group member map
// with Entra group ID as the map key.
type groupMembersByGroupID map[entraUniqueID][]models.GroupMember

type entraGroups struct {
	groupsMap       groupsByID
	groupMembersMap groupMembersByGroupID
}

func (g entraGroups) toAccessListsWithMembers(
	ctx context.Context,
	tenantID string,
	usersByEntraID map[entraUniqueID]types.User,
	aclOwnersCfg aclOwnersConfig,
) (map[string]*accessListWithMembers, errSkippedResources) {
	var errSkippedResources errSkippedResources
	aclsWithMembersMap := make(map[string]*accessListWithMembers)
	accessListsById := make(map[entraUniqueID]*accesslist.AccessList)
	var errGroups, errGroupMembers error

	for _, group := range g.groupsMap {
		owners := aclOwnersCfg.getOwners(ctx, group, usersByEntraID)
		entraUniqueID, al, err := convertGroup(group, tenantID, owners)
		if err != nil {
			errGroups = errors.Join(errGroups, trace.Wrap(err))
			continue
		}
		aclsWithMembersMap[al.GetName()] = &accessListWithMembers{AccessList: al}
		accessListsById[entraUniqueID] = al
	}

	var notFoundMembers []string
	for id, accessList := range accessListsById {
		var members []*accesslist.AccessListMember
		for _, member := range g.groupMembersMap[id] {
			m, err := convertGroupMember(member, accessList, usersByEntraID, accessListsById)
			if err != nil {
				if trace.IsNotFound(err) {
					notFoundMembers = append(notFoundMembers, strval(member.GetID()))
				} else {
					errGroupMembers = errors.Join(errGroupMembers, trace.Wrap(err))
				}
				continue
			}
			if m == nil {
				slog.WarnContext(ctx, "Unsupported Entra ID group member, skipping", "group_id", id)
				continue
			}
			members = append(members, m)
		}
		aclsWithMembersMap[accessList.GetName()].Members = members
	}

	if errGroups != nil {
		errSkippedResources.groups = append(errSkippedResources.groups, errGroups)
	}
	if errGroupMembers != nil {
		errSkippedResources.groupMembers = append(errSkippedResources.groupMembers, errGroupMembers)
	}
	if len(notFoundMembers) > 0 {
		members := utils.Deduplicate(notFoundMembers)
		errSkippedResources.groupMembers = append(errSkippedResources.groupMembers,
			trace.NotFound("member(s) not found and may have been filtered or skipped due to error. Member IDs: %s", strings.Join(members, ", ")))
	}

	return aclsWithMembersMap, errSkippedResources
}

func convertGroup(in *models.Group, tenantID string, owners []accesslist.Owner) (entraUniqueID, *accesslist.AccessList, error) {
	if err := validateGroup(in); err != nil {
		return "", nil, trace.Wrap(err)
	}
	displayName := *in.DisplayName
	id := *in.ID

	out, err := accesslist.NewAccessList(
		header.Metadata{
			Name: accessListName(displayName, id),
		},
		accesslist.Spec{
			Title:  displayName,
			Owners: owners,
			Grants: accesslist.Grants{
				Traits: trait.Traits{
					eteleport.EntraMemberOfGroupTrait: {id},
				},
			},
		},
	)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	staticLabels := map[string]string{
		types.EntraTenantIDLabel:    tenantID,
		types.EntraUniqueIDLabel:    id,
		types.EntraDisplayNameLabel: displayName,
	}

	// The logic to construct user trait from their group membership
	// is based on enterprise application settings and depends on the
	// group OnPremisesDomainName, OnPremisesNetBiosName, OnPremisesSamAccountName
	// properties. These values are preserved in Access List labels so
	// that the delta sync can apply the trait changes to user resource
	// if it detects changes in the enterprise application settings.
	// See [getGroupNameBuilderFunc] and [buildUserMemberships] functions.
	if in.OnPremisesDomainName != nil {
		staticLabels[onPremisesDomainNameLabel] = *in.OnPremisesDomainName
	}
	if in.OnPremisesNetBiosName != nil {
		staticLabels[onPremisesNetBiosNameLabel] = *in.OnPremisesNetBiosName
	}
	if in.OnPremisesSamAccountName != nil {
		staticLabels[onPremisesSamAccountNameLabel] = *in.OnPremisesSamAccountName
	}
	if len(in.GroupTypes) != 0 {
		staticLabels[groupTypeLabel] = strings.Join(in.GroupTypes, ",")
	}
	out.SetStaticLabels(staticLabels)
	out.SetOrigin(types.OriginEntraID)
	return entraUniqueID(id), out, nil
}

// convertGroupMember converts an Entra group member to an AccessListMember.
// Error is returned on unexpected conditions, indicating programmer error (e.g. validation of AccessListMember fails).
// On non fatal errors, e.g. an unsupported member type, a warning is logged and (nil, nil) is returned.
func convertGroupMember(
	in models.GroupMember,
	al *accesslist.AccessList,
	entraUsersByID map[entraUniqueID]types.User,
	accesslistByEntraId map[entraUniqueID]*accesslist.AccessList,
) (*accesslist.AccessListMember, error) {
	if in.GetID() == nil {
		return nil, trace.BadParameter("expected Entra ID group member to have a non-empty unique ID")
	}
	id := *in.GetID()

	switch m := in.(type) {
	case *models.User:
		teleportUser, ok := entraUsersByID[entraUniqueID(id)]
		if !ok {
			return nil, trace.NotFound("group member account found for Entra unique ID %q", id)
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
				// Users imported from Entra ID are always eligible since Entra ID access lists
				// do not have membership expiration or eligibility requirements.
				// Setting IneligibleStatus to ELIGIBLE allows the reconciler to skip
				// unnecessary ineligibility updates, improving performance.
				IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	case *models.Group:
		accessList, ok := accesslistByEntraId[entraUniqueID(id)]
		if !ok {
			return nil, trace.NotFound("access list not found for Entra unique ID %q", id)
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
				// Nested Access Lists members imported from Entra ID are always eligible.
				// Setting IneligibleStatus to ELIGIBLE allows the reconciler to skip
				// unnecessary ineligibility updates, improving performance.
				IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	default:
		return nil, trace.BadParameter("entra group member(id=%s) expected to be of user or group type, got %T", id, m)
	}
}

type accessListWithMembers struct {
	*accesslist.AccessList
	Members []*accesslist.AccessListMember
}

func (a *accessListWithMembers) isEqual(other *accessListWithMembers) bool {
	// Skip cloning during comparison since reconciliation inputs are ephemeral and
	// recreated on each run. This optimization avoids unnecessary allocations by
	// allowing in-place mutations that would be discarded after reconciliation anyway.
	if !accesslist.EqualAccessLists(a.AccessList, other.AccessList,
		accesslist.WithSkipClone(),
		accesslist.WithIgnoreEphemeralFields()) {
		return false
	}
	if len(a.Members) != len(other.Members) {
		return false
	}
	// Members are sorted before reconciliation, so we can compare them in order.
	for i := range a.Members {
		if a.Members[i].Spec.Name != other.Members[i].Spec.Name ||
			a.Members[i].Spec.AccessList != other.Members[i].Spec.AccessList {
			return false
		}
	}
	return true
}

// GetKind returns a fake resource kind printed in the [services.Reconciler] logs.
func (a *accessListWithMembers) GetKind() string {
	return types.KindAccessList + "+" + types.KindAccessListMember
}

type aclOwnersConfig struct {
	defaultOwners []accesslist.Owner
	source        types.EntraIDAccessListOwnersSource
}

func (cfg aclOwnersConfig) getOwners(ctx context.Context, group *models.Group, usersByEntraID map[entraUniqueID]types.User) []accesslist.Owner {
	if cfg.source == types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN {
		return cfg.defaultOwners
	}

	out := toAclOwner(ctx, group.Owners, usersByEntraID)
	if len(out) == 0 {
		slog.DebugContext(ctx, `Empty group owners found when Entra ID is configured as the source of the Access List owner, `+
			`falling back to default owners`, "group_id", group.GetID())
		return cfg.defaultOwners
	}

	switch cfg.source {
	case types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID:
		return out
	case types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID:
		return slices.Concat(out, cfg.defaultOwners)
	default:
		// Unknown source should fallback to default owners for backward compatibility.
		return cfg.defaultOwners
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

func unwindGroupMembership(in entraGroups) map[string][]string {
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
		for _, member := range in.groupMembersMap[entraUniqueID(groupID)] {
			if nestedGroup, ok := member.(*models.Group); ok {
				// Skip Office 365 groups, we only care about security groups.

				// In delta sync, a group is constructed from the corresponding Access List
				// resource and its group member is constructured from the corresponding
				// Access List member resource. Access List resources persist group type label
				// but member resources do not. So IsOffice365Group is checked against the group
				// in `groupsMap` which is a canonical map of all the parent and nested groups.
				// This works for both full and delta sync scenario.
				id := *nestedGroup.ID
				group, ok := in.groupsMap[entraUniqueID(id)]
				if !ok || group.IsOffice365Group() {
					continue
				}
				// recursively collect the path for nested groups.
				nestedPath := collectPaths(id)
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
	for groupID, group := range in.groupsMap {
		// Skip Office 365 groups, we only care about security groups.
		if group.IsOffice365Group() {
			continue
		}
		collectPaths(string(groupID))
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

func sortMembers(m map[string]*accessListWithMembers) {
	for _, a := range m {
		slices.SortFunc(a.Members, func(x, y *accesslist.AccessListMember) int {
			return strings.Compare(x.GetName(), y.GetName())
		})
	}
}

type memberKey struct {
	Name       string
	AccessList string
}

func newMemberKey(m *accesslist.AccessListMember) memberKey {
	return memberKey{
		Name:       m.GetName(),
		AccessList: m.Spec.AccessList,
	}
}

func preserveFields(dst, src map[string]*accessListWithMembers) {
	for k, dst := range dst {
		if src, ok := src[k]; ok {
			preserveAccessListFields(dst.AccessList, src.AccessList)
		}
	}

	dstMembersMap := make(map[memberKey]*accesslist.AccessListMember)
	for _, a := range dst {
		for _, m := range a.Members {
			dstMembersMap[newMemberKey(m)] = m
		}
	}
	for _, a := range src {
		for _, src := range a.Members {
			if dst, ok := dstMembersMap[newMemberKey(src)]; ok {
				preserveAccessListMemberFields(dst, src)
			}
		}
	}
}

func preserveAccessListFields(dst, src *accesslist.AccessList) {
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

func preserveAccessListMemberFields(dst, src *accesslist.AccessListMember) {
	dst.Spec.Joined = src.Spec.Joined
}

func strval(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func toCollection(in map[string]*accessListWithMembers) (*accesslists.Collection, error) {
	c := accesslists.Collection{}
	for _, v := range in {
		if err := c.AddAccessList(v.AccessList, v.Members); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &c, nil
}

func groupNameForLog(in *models.Group) string {
	if in == nil {
		return ""
	}
	if in.GetID() != nil {
		return fmt.Sprintf("(id=%s)", *in.GetID())
	}
	if in.DisplayName != nil {
		return fmt.Sprintf("(displayName=%s)", *in.DisplayName)
	}
	return ""
}

// toAclOwner converts models.User to accesslist.Owner.
func toAclOwner(ctx context.Context, in []*models.User, usersByEntraID map[entraUniqueID]types.User) []accesslist.Owner {
	out := make([]accesslist.Owner, 0, len(in))
	for _, u := range in {
		if u == nil || u.GetID() == nil || *u.GetID() == "" {
			continue
		}

		// Entra user missing from usersByEntraID should not be added as owners
		// as it may have been filtered.
		entraUser, ok := usersByEntraID[entraUniqueID(*u.GetID())]
		if !ok {
			slog.DebugContext(ctx,
				"Teleport user account not found for Entra ID group owner, owner will be skipped",
				"user_id", *u.GetID(),
			)
			continue
		}

		// In delta sync, incoming owner from group object may only
		// contain user object ID and will fail processUsername.
		// But converting to owner from usersByEntraID works for
		// both full and delta sync as it is a collection
		// of Teleport user resource derived properly from Entra users.
		owner := entraOwnerFromTeleportUser(entraUser, *u.GetID())
		username, _, err := processUsername(owner)
		if err != nil {
			slog.WarnContext(ctx,
				"Failed to convert group owner, owner will be skipped",
				"user_id", *u.GetID(),
				"error", err,
			)
			continue
		}

		out = append(out, accesslist.Owner{
			Name:           username,
			MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
			// Set IneligibleStatus to ELIGIBLE for user owners.
			// This optimizes the reconciler by skipping ineligibility checks,
			// since Entra ID access lists do not have owner eligibility requirements.
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		})
	}

	slices.SortFunc(out, func(a, b accesslist.Owner) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out
}

// deleteNestedAccessLists ranges through [accesslist.MaxAllowedDepth] and retries
// deletion on each [accesslists.ErrDeniedAccessListDeletion] error returned for
// the Access List. This is necessary because for nested Access List, deletion is
// prevented until the parent Access List itslef is deleted or until the nested
// membership is removed. Looping for [accesslist.MaxAllowedDepth] counter prevents
// running the deletion loop forever.
func deleteNestedAccessLists(ctx context.Context, accessPoint accessPoint, aclToDelete []string) error {
	var errs []error
	pending := aclToDelete

	for range accesslist.MaxAllowedDepth {
		idx := 0
		for _, name := range pending {
			err := accessPoint.DeleteAccessList(ctx, name)
			if err == nil || trace.IsNotFound(err) {
				continue
			}
			if errors.Is(err, accesslists.ErrDeniedAccessListDeletion) {
				pending[idx] = name
				idx++
				continue
			}
			errs = append(errs, err)
		}
		pending = pending[:idx]
		if len(pending) == 0 {
			return trace.NewAggregate(errs...)
		}
	}

	// Do a last deletion pass before bailing out.
	for _, name := range pending {
		if err := accessPoint.DeleteAccessList(ctx, name); err != nil {
			if !trace.IsNotFound(err) {
				errs = append(errs, err)
			}
		}
	}

	return trace.NewAggregate(errs...)
}

// filterOutCyclicMemberships mutates `entraAccessList` by removing members that
// introduce cycles, e.g. listA -> listB and listB -> listA. Filtering is deterministic
// because Access Lists and members are sorted.
//
// On the first sync where there are no Entra ID Access List synced to Teleport,
// given Access Lists listA and listB with cyclic membership:
//
//	listA -> listB
//	listB -> listA
//
// listA is evaluated before listB, so listA -> listB is accepted first.
// listB -> listA introduces cycle and is removed.
//
// On a subsequent sync where Entra ID Access List and membership already exist
// in Teleport, Access Lists and members are sorted, but existing membership wins.
// For example, given listB -> listA membership exists in Teleport, and
//
//	listB -> listA
//	listA -> listB
//
// is being synced from Entra ID, existing membership listB -> listA wins and
// listA -> listB is removed.
func filterOutCyclicMemberships(ctx context.Context, entraAccessList, teleportAccessList map[string]*accessListWithMembers) error {
	// Access List and member collection used as a base state during validation.
	coll := &accesslists.Collection{
		AccessListsByName: make(map[string]*accesslist.AccessList, len(entraAccessList)),
		// MembersByAccessList holds validated members. This is needed to detect the
		// member edge that introduces cycle and not remove the existing valid membership.
		MembersByAccessList: make(map[string][]*accesslist.AccessListMember, len(entraAccessList)),
	}

	// Sort members to make cyclic membership filter deterministic.
	sortMembers(entraAccessList)
	for name, a := range entraAccessList {
		coll.AccessListsByName[name] = a.AccessList
	}

	aclNames := sortedAccessListNames(entraAccessList)
	teleportMemberEdge := newMemberEdgeSet(teleportAccessList)
	if len(teleportMemberEdge) > 0 {
		// Seed existing Entra ID Access List memberships.
		// This is necessary so that ValidateAccessListMember sees all the membership
		// edges that already exists in Teleport.
		for _, aclName := range aclNames {
			for _, member := range entraAccessList[aclName].Members {
				if teleportMemberEdge.contains(aclName, member.GetName()) {
					coll.MembersByAccessList[aclName] = append(coll.MembersByAccessList[aclName], member)
				}
			}
		}
	}

	var skippedMembers []error
	for _, aclName := range aclNames {
		entraAcl := entraAccessList[aclName]
		currentMembers := coll.MembersByAccessList[aclName]

		for _, member := range entraAcl.Members {
			if teleportMemberEdge.contains(aclName, member.GetName()) {
				// Skip validation for this edge because existing edge wins (and is preserved)
				// over the new membership edge that introduces cycle.
				continue
			}

			if err := accesslists.ValidateAccessListMember(ctx, entraAcl.AccessList, member, coll); err != nil {
				if errors.Is(err, accesslists.ErrCyclicMembership) {
					skippedMembers = append(skippedMembers, err)
					continue
				}
				return trace.Wrap(err)
			}
			currentMembers = append(currentMembers, member)
			// Update collection so the next validation edge sees the current
			// membership edge.
			coll.MembersByAccessList[aclName] = currentMembers
		}

		if currentMembers == nil {
			// Instantiate if all the members for this Access List were filtered.
			currentMembers = entraAcl.Members[:0]
		}
		entraAcl.Members = currentMembers
	}

	return trace.NewAggregate(skippedMembers...)
}

// sortedAccessListNames sorts Access List with their titles first and
// falls back to sorting with resource names as a tie breaker.
func sortedAccessListNames(in map[string]*accessListWithMembers) []string {
	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		if cmp := strings.Compare(in[a].Spec.Title, in[b].Spec.Title); cmp != 0 {
			return cmp
		}
		return strings.Compare(a, b)
	})
	return names
}

type nestedEdge struct {
	accessList string
	member     string
}

type nestedEdgeSet map[nestedEdge]struct{}

func newMemberEdgeSet(in map[string]*accessListWithMembers) nestedEdgeSet {
	out := make(nestedEdgeSet)

	for aclName, acl := range in {
		for _, member := range acl.Members {
			if member.Spec.MembershipKind != accesslist.MembershipKindList {
				continue
			}
			out[nestedEdge{
				accessList: aclName,
				member:     member.GetName(),
			}] = struct{}{}
		}
	}

	return out
}

func (s nestedEdgeSet) contains(acl, member string) bool {
	_, ok := s[nestedEdge{
		accessList: acl,
		member:     member,
	}]
	return ok
}
