/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accesslists

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/services"
)

// RelationshipKind represents the type of relationship: member or owner.
type RelationshipKind int

const (
	RelationshipKindMember RelationshipKind = iota
	RelationshipKindOwner
)

// MembershipOrOwnershipType represents the type of membership or ownership a User has for an Access List.
type MembershipOrOwnershipType int

const (
	// MembershipOrOwnershipTypeNone indicates that the User lacks valid Membership or Ownership for the Access List.
	MembershipOrOwnershipTypeNone MembershipOrOwnershipType = iota
	// MembershipOrOwnershipTypeExplicit indicates that the User has explicit Membership or Ownership for the Access List.
	MembershipOrOwnershipTypeExplicit
	// MembershipOrOwnershipTypeInherited indicates that the User has inherited Membership or Ownership for the Access List.
	MembershipOrOwnershipTypeInherited
)

// AccessListAndMembersGetter is a minimal interface for fetching AccessLists by name, and AccessListMembers for an Access List.
type AccessListAndMembersGetter interface {
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error)
}

// GetMembersFor returns a flattened list of Members for an Access List, including inherited Members.
//
// Returned Members are not validated for expiration or other requirements – use IsAccessListMember
// to validate a Member's membership status.
func GetMembersFor(ctx context.Context, accessListName string, g AccessListAndMembersGetter) ([]*accesslist.AccessListMember, error) {
	return getMembersFor(ctx, accessListName, g, make(map[string]struct{}))
}

func getMembersFor(ctx context.Context, accessListName string, g AccessListAndMembersGetter, visited map[string]struct{}) ([]*accesslist.AccessListMember, error) {
	if _, ok := visited[accessListName]; ok {
		return nil, nil
	}
	visited[accessListName] = struct{}{}

	members, err := fetchMembers(ctx, accessListName, g)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var allMembers []*accesslist.AccessListMember
	for _, member := range members {
		if member.Spec.MembershipKind != accesslist.MembershipKindList {
			allMembers = append(allMembers, member)
			continue
		}
		childMembers, err := getMembersFor(ctx, member.Spec.Name, g, visited)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allMembers = append(allMembers, childMembers...)
	}

	return allMembers, nil
}

// fetchMembers is a simple helper to fetch all top-level AccessListMembers for an AccessList.
func fetchMembers(ctx context.Context, accessListName string, g AccessListAndMembersGetter) ([]*accesslist.AccessListMember, error) {
	var allMembers []*accesslist.AccessListMember
	pageToken := ""
	for {
		page, nextToken, err := g.ListAccessListMembers(ctx, accessListName, 0, pageToken)
		if err != nil {
			// If the AccessList doesn't exist yet, should return an empty list of members
			if trace.IsNotFound(err) {
				break
			}
			return nil, trace.Wrap(err)
		}
		allMembers = append(allMembers, page...)
		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}
	return allMembers, nil
}

// ValidateAccessListWithMembers validates a new or existing AccessList with a list of AccessListMembers.
func ValidateAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, members []*accesslist.AccessListMember, g AccessListAndMembersGetter) error {
	for _, owner := range accessList.Spec.Owners {
		if owner.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		ownerList, err := g.GetAccessList(ctx, owner.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := validateAddition(ctx, accessList, ownerList, RelationshipKindOwner, g); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, member := range members {
		if member.Spec.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		memberList, err := g.GetAccessList(ctx, member.Spec.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := validateAddition(ctx, accessList, memberList, RelationshipKindMember, g); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// collectOwners is a helper to recursively collect all Owners for an Access List, including inherited Owners.
func collectOwners(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter, owners map[string]*accesslist.Owner, visited map[string]struct{}) error {
	//owners := make([]*accesslist.Owner, 0)
	if _, ok := visited[accessList.GetName()]; ok {
		return nil
	}
	visited[accessList.GetName()] = struct{}{}

	for _, owner := range accessList.Spec.Owners {
		if owner.MembershipKind != accesslist.MembershipKindList {
			// Collect direct owner users
			owners[owner.Name] = &owner
			continue
		}

		// For owner lists, we need to collect their members as owners
		ownerMembers, err := collectMembersAsOwners(ctx, owner.Name, g, visited)
		if err != nil {
			return trace.Wrap(err)
		}
		//owners = append(owners, ownerMembers...)
		for _, ownerMember := range ownerMembers {
			owners[ownerMember.Name] = ownerMember
		}
	}

	return nil
}

// collectMembersAsOwners is a helper to collect all nested members of an AccessList and return them cast as Owners.
func collectMembersAsOwners(ctx context.Context, accessListName string, g AccessListAndMembersGetter, visited map[string]struct{}) ([]*accesslist.Owner, error) {
	owners := make([]*accesslist.Owner, 0)
	if _, ok := visited[accessListName]; ok {
		return owners, nil
	}
	visited[accessListName] = struct{}{}

	listMembers, err := GetMembersFor(ctx, accessListName, g)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, member := range listMembers {
		owners = append(owners, &accesslist.Owner{
			Name:             member.Spec.Name,
			Description:      member.Metadata.Description,
			IneligibleStatus: "",
			MembershipKind:   accesslist.MembershipKindUser,
		})
	}

	return owners, nil
}

// GetOwnersFor returns a flattened list of Owners for an Access List, including inherited Owners.
//
// Returned Owners are not validated for expiration or other requirements – use IsAccessListOwner
// to validate an Owner's ownership status.
func GetOwnersFor(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter) ([]*accesslist.Owner, error) {
	ownersMap := make(map[string]*accesslist.Owner)
	if err := collectOwners(ctx, accessList, g, ownersMap, make(map[string]struct{})); err != nil {
		return nil, trace.Wrap(err)
	}
	owners := make([]*accesslist.Owner, 0, len(ownersMap))
	for _, owner := range ownersMap {
		owners = append(owners, owner)
	}
	return owners, nil
}

// ValidateAccessListMember validates a new or existing AccessListMember for an Access List.
func ValidateAccessListMember(
	ctx context.Context,
	parentList *accesslist.AccessList,
	member *accesslist.AccessListMember,
	g AccessListAndMembersGetter,
) error {
	if member.Spec.MembershipKind != accesslist.MembershipKindList {
		return nil
	}
	return validateAccessListMemberOrOwner(ctx, parentList, member.Spec.Name, RelationshipKindMember, g)
}

// ValidateAccessListOwner validates a new or existing AccessListOwner for an Access List.
func ValidateAccessListOwner(
	ctx context.Context,
	parentList *accesslist.AccessList,
	owner *accesslist.Owner,
	g AccessListAndMembersGetter,
) error {
	if owner.MembershipKind != accesslist.MembershipKindList {
		return nil
	}
	return validateAccessListMemberOrOwner(ctx, parentList, owner.Name, RelationshipKindOwner, g)
}

func validateAccessListMemberOrOwner(
	ctx context.Context,
	parentList *accesslist.AccessList,
	memberOrOwnerName string,
	kind RelationshipKind,
	g AccessListAndMembersGetter,
) error {
	// Ensure member or owner list exists
	memberOrOwnerList, err := g.GetAccessList(ctx, memberOrOwnerName)
	if err != nil {
		return trace.Wrap(err)
	}

	// Validate addition
	if err := validateAddition(ctx, parentList, memberOrOwnerList, kind, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateAddition(
	ctx context.Context,
	parentList *accesslist.AccessList,
	childList *accesslist.AccessList,
	kind RelationshipKind,
	g AccessListAndMembersGetter,
) error {
	kindStr := "a Member"
	if kind == RelationshipKindOwner {
		kindStr = "an Owner"
	}

	// Cycle detection
	reachable, err := isReachable(ctx, childList, parentList, make(map[string]struct{}), g)
	if err != nil {
		return trace.Wrap(err)
	}
	if reachable {
		return trace.BadParameter(
			"Access List '%s' can't be added as %s of '%s' because '%s' is already included as a Member or Owner in '%s'",
			childList.Spec.Title, kindStr, parentList.Spec.Title, parentList.Spec.Title, childList.Spec.Title)
	}

	// Max depth check
	exceeds, err := exceedsMaxDepth(ctx, parentList, childList, kind, g)
	if err != nil {
		return trace.Wrap(err)
	}
	if exceeds {
		return trace.BadParameter(
			"Access List '%s' can't be added as %s of '%s' because it would exceed the maximum nesting depth of %d",
			childList.Spec.Title, kindStr, parentList.Spec.Title, accesslist.MaxAllowedDepth)
	}

	return nil
}

func isReachable(
	ctx context.Context,
	currentList *accesslist.AccessList,
	targetList *accesslist.AccessList,
	visited map[string]struct{},
	g AccessListAndMembersGetter,
) (bool, error) {
	if currentList.GetName() == targetList.GetName() {
		return true, nil
	}
	if _, ok := visited[currentList.GetName()]; ok {
		return false, nil
	}
	visited[currentList.GetName()] = struct{}{}

	// Traverse member lists
	listMembers, err := fetchMembers(ctx, currentList.GetName(), g)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, member := range listMembers {
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			childList, err := g.GetAccessList(ctx, member.Spec.Name)
			if err != nil {
				return false, trace.Wrap(err)
			}
			reachable, err := isReachable(ctx, childList, targetList, visited, g)
			if err != nil {
				return false, trace.Wrap(err)
			}
			if reachable {
				return true, nil
			}
		}
	}

	// Traverse owner lists
	for _, owner := range currentList.Spec.Owners {
		if owner.MembershipKind == accesslist.MembershipKindList {
			ownerList, err := g.GetAccessList(ctx, owner.Name)
			if err != nil {
				return false, trace.Wrap(err)
			}
			reachable, err := isReachable(ctx, ownerList, targetList, visited, g)
			if err != nil {
				return false, trace.Wrap(err)
			}
			if reachable {
				return true, nil
			}
		}
	}

	return false, nil
}

func exceedsMaxDepth(
	ctx context.Context,
	parentList *accesslist.AccessList,
	childList *accesslist.AccessList,
	kind RelationshipKind,
	g AccessListAndMembersGetter,
) (bool, error) {
	switch kind {
	case RelationshipKindOwner:
		// For Owners, only consider the depth downwards from the child node
		depthDownwards, err := maxDepthDownwards(ctx, childList.GetName(), make(map[string]struct{}), g)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return depthDownwards > accesslist.MaxAllowedDepth, nil
	default:
		// For Members, consider the depth upwards from the parent node, downwards from the child node, and the edge between them
		depthUpwards, err := maxDepthUpwards(ctx, parentList, make(map[string]struct{}), g)
		if err != nil {
			return false, trace.Wrap(err)
		}
		depthDownwards, err := maxDepthDownwards(ctx, childList.GetName(), make(map[string]struct{}), g)
		if err != nil {
			return false, trace.Wrap(err)
		}
		totalDepth := depthUpwards + depthDownwards + 1 // +1 for the edge between parent and child
		return totalDepth > accesslist.MaxAllowedDepth, nil
	}
}

func maxDepthDownwards(
	ctx context.Context,
	currentListName string,
	seen map[string]struct{},
	g AccessListAndMembersGetter,
) (int, error) {
	if _, ok := seen[currentListName]; ok {
		return 0, nil
	}
	seen[currentListName] = struct{}{}

	maxDepth := 0

	listMembers, err := fetchMembers(ctx, currentListName, g)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	for _, member := range listMembers {
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			childListName := member.Spec.Name
			depth, err := maxDepthDownwards(ctx, childListName, seen, g)
			if err != nil {
				return 0, trace.Wrap(err)
			}
			depth += 1 // Edge to the child
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	delete(seen, currentListName)

	return maxDepth, nil
}

func maxDepthUpwards(
	ctx context.Context,
	currentList *accesslist.AccessList,
	seen map[string]struct{},
	g AccessListAndMembersGetter,
) (int, error) {
	if _, ok := seen[currentList.GetName()]; ok {
		return 0, nil
	}
	seen[currentList.GetName()] = struct{}{}

	maxDepth := 0

	// Traverse MemberOf relationships
	for _, parentListName := range currentList.Status.MemberOf {
		parentList, err := g.GetAccessList(ctx, parentListName)
		if err != nil {
			return 0, trace.Wrap(err) // Treat missing lists as depth 0
		}
		depth, err := maxDepthUpwards(ctx, parentList, seen, g)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		depth += 1 // Edge to the parent
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	delete(seen, currentList.GetName())

	return maxDepth, nil
}

func IsAccessListOwner(
	ctx context.Context,
	user types.User,
	accessList *accesslist.AccessList,
	g AccessListAndMembersGetter,
	lockGetter services.LockGetter,
	clock clockwork.Clock,
) (MembershipOrOwnershipType, error) {
	if lockGetter != nil {
		locks, err := lockGetter.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return MembershipOrOwnershipTypeNone, trace.Wrap(err)
		}
		if len(locks) > 0 {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' is currently locked", user.GetName())
		}
	}

	var ownershipErr error

	for _, owner := range accessList.Spec.Owners {
		// Is user an explicit owner?
		if owner.MembershipKind != accesslist.MembershipKindList && owner.Name == user.GetName() {
			if !UserMeetsRequirements(user, accessList.Spec.OwnershipRequires) {
				// Avoid non-deterministic behavior in these checks. Rather than returning immediately, continue
				// through all owners to make sure there isn't a valid match later on.
				ownershipErr = trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
				continue
			}
			return MembershipOrOwnershipTypeExplicit, nil
		}
		// Is user an inherited owner through any potential owner AccessLists?
		if owner.MembershipKind == accesslist.MembershipKindList {
			ownerAccessList, err := g.GetAccessList(ctx, owner.Name)
			if err != nil {
				ownershipErr = trace.Wrap(err)
				continue
			}
			// Since we already verified that the user is not locked, don't provide lockGetter here
			membershipType, err := IsAccessListMember(ctx, user, ownerAccessList, g, nil, clock)
			if err != nil {
				ownershipErr = trace.Wrap(err)
				continue
			}
			if membershipType != MembershipOrOwnershipTypeNone {
				if !UserMeetsRequirements(user, accessList.Spec.OwnershipRequires) {
					ownershipErr = trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
					continue
				}
				return MembershipOrOwnershipTypeInherited, nil
			}
		}
	}

	return MembershipOrOwnershipTypeNone, trace.Wrap(ownershipErr)
}

func IsAccessListMember(
	ctx context.Context,
	user types.User,
	accessList *accesslist.AccessList,
	g AccessListAndMembersGetter,
	lockGetter services.LockGetter,
	clock clockwork.Clock,
) (MembershipOrOwnershipType, error) {
	if lockGetter != nil {
		locks, err := lockGetter.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return MembershipOrOwnershipTypeNone, trace.Wrap(err)
		}
		if len(locks) > 0 {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' is currently locked", user.GetName())
		}
	}

	members, err := fetchMembers(ctx, accessList.GetName(), g)
	if err != nil {
		return MembershipOrOwnershipTypeNone, trace.Wrap(err)
	}

	var membershipErr error

	for _, member := range members {
		// Is user an explicit member?
		if member.Spec.MembershipKind != accesslist.MembershipKindList && member.GetName() == user.GetName() {
			if !UserMeetsRequirements(user, accessList.Spec.MembershipRequires) {
				// Avoid non-deterministic behavior in these checks. Rather than returning immediately, continue
				// through all members to make sure there isn't a valid match later on.
				membershipErr = trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
				continue
			}
			if !member.Spec.Expires.IsZero() && !clock.Now().Before(member.Spec.Expires) {
				membershipErr = trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), accessList.Spec.Title)
				continue
			}
			return MembershipOrOwnershipTypeExplicit, nil
		}
		// Is user an inherited member through any potential member AccessLists?
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			memberAccessList, err := g.GetAccessList(ctx, member.GetName())
			if err != nil {
				membershipErr = trace.Wrap(err)
				continue
			}
			// Since we already verified that the user is not locked, don't provide lockGetter here
			membershipType, err := IsAccessListMember(ctx, user, memberAccessList, g, nil, clock)
			if err != nil {
				membershipErr = trace.Wrap(err)
				continue
			}
			if membershipType != MembershipOrOwnershipTypeNone {
				if !UserMeetsRequirements(user, accessList.Spec.MembershipRequires) {
					membershipErr = trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
					continue
				}
				if !member.Spec.Expires.IsZero() && !clock.Now().Before(member.Spec.Expires) {
					membershipErr = trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), accessList.Spec.Title)
					continue
				}
				return MembershipOrOwnershipTypeInherited, nil
			}
		}
	}

	return MembershipOrOwnershipTypeNone, trace.Wrap(membershipErr)
}

// UserMeetsRequirements is a helper which will return whether the User meets the AccessList Ownership/MembershipRequires.
func UserMeetsRequirements(identity types.User, requires accesslist.Requires) bool {
	// Assemble the user's roles for easy look up.
	userRolesMap := map[string]struct{}{}
	for _, role := range identity.GetRoles() {
		userRolesMap[role] = struct{}{}
	}

	// Check that the user meets the role requirements.
	for _, role := range requires.Roles {
		if _, ok := userRolesMap[role]; !ok {
			return false
		}
	}

	// Assemble traits for easy lookup.
	userTraitsMap := map[string]map[string]struct{}{}
	for k, values := range identity.GetTraits() {
		if _, ok := userTraitsMap[k]; !ok {
			userTraitsMap[k] = map[string]struct{}{}
		}

		for _, v := range values {
			userTraitsMap[k][v] = struct{}{}
		}
	}

	// Check that user meets trait requirements.
	for k, values := range requires.Traits {
		if _, ok := userTraitsMap[k]; !ok {
			return false
		}

		for _, v := range values {
			if _, ok := userTraitsMap[k][v]; !ok {
				return false
			}
		}
	}

	// The user meets all requirements.
	return true
}

// GetAncestorsFor calculates and returns the set of Ancestor ACLs depending on
// the supplied relationship criteria. Order of the ancestor list is undefined.
func GetAncestorsFor(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, g AccessListAndMembersGetter, log *slog.Logger) ([]*accesslist.AccessList, error) {
	ancestorsMap := make(map[string]*accesslist.AccessList)
	if err := collectAncestors(ctx, accessList, kind, g, log, make(map[string]struct{}), ancestorsMap); err != nil {
		return nil, trace.Wrap(err)
	}
	ancestors := slices.Collect(maps.Values(ancestorsMap))
	return ancestors, nil
}

func collectAncestors(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, g AccessListAndMembersGetter, log *slog.Logger, visited map[string]struct{}, ancestors map[string]*accesslist.AccessList) error {
	if _, ok := visited[accessList.GetName()]; ok {
		return nil
	}
	visited[accessList.GetName()] = struct{}{}

	switch kind {
	case RelationshipKindOwner:
		// Add parents where this list is an owner to ancestors
		for _, ownerParent := range accessList.Status.OwnerOf {
			ownerParentAcl, err := g.GetAccessList(ctx, ownerParent)
			if err != nil {
				log.DebugContext(ctx, "Failed to get parent Access List", "access_list_id", ownerParent, "error", err)

				// If parent list doesn't exist, may be recently deleted, so ignore
				if trace.IsNotFound(err) {
					continue
				}
				return trace.Wrap(err)
			}
			ancestors[ownerParent] = ownerParentAcl
		}
		// Recursively traverse parents where this list is a member
		for _, memberParent := range accessList.Status.MemberOf {
			memberParentAcl, err := g.GetAccessList(ctx, memberParent)
			if err != nil {
				log.DebugContext(ctx, "Failed to get parent Access List", "access_list_id", memberParent, "error", err)

				// If parent list doesn't exist, may be recently deleted, so ignore
				if trace.IsNotFound(err) {
					continue
				}
				return trace.Wrap(err)
			}
			if err := collectAncestors(ctx, memberParentAcl, kind, g, log, visited, ancestors); err != nil {
				return trace.Wrap(err)
			}
		}
	default:
		// Only collect parents where this list is a member
		for _, memberParent := range accessList.Status.MemberOf {
			memberParentAcl, err := g.GetAccessList(ctx, memberParent)
			if err != nil {
				return trace.Wrap(err)
			}
			ancestors[memberParent] = memberParentAcl
			if err := collectAncestors(ctx, memberParentAcl, kind, g, log, visited, ancestors); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// GetInheritedGrants returns the combined Grants for an Access List's members, inherited from any ancestor lists.
func GetInheritedGrants(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter, log *slog.Logger) (*accesslist.Grants, error) {
	grants := accesslist.Grants{
		Traits: trait.Traits{},
	}

	collectedRoles := make(map[string]struct{})
	collectedTraits := make(map[string]map[string]struct{})

	addGrants := func(grantRoles []string, grantTraits trait.Traits) {
		for _, role := range grantRoles {
			if _, exists := collectedRoles[role]; !exists {
				grants.Roles = append(grants.Roles, role)
				collectedRoles[role] = struct{}{}
			}
		}
		for traitKey, traitValues := range grantTraits {
			if _, exists := collectedTraits[traitKey]; !exists {
				collectedTraits[traitKey] = make(map[string]struct{})
			}
			for _, traitValue := range traitValues {
				if _, exists := collectedTraits[traitKey][traitValue]; !exists {
					grants.Traits[traitKey] = append(grants.Traits[traitKey], traitValue)
					collectedTraits[traitKey][traitValue] = struct{}{}
				}
			}
		}
	}

	// Get ancestors via member relationship
	ancestorLists, err := GetAncestorsFor(ctx, accessList, RelationshipKindMember, g, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ancestor := range ancestorLists {
		memberGrants := ancestor.GetGrants()
		addGrants(memberGrants.Roles, memberGrants.Traits)
	}

	// Get ancestors via owner relationship
	ancestorOwnerLists, err := GetAncestorsFor(ctx, accessList, RelationshipKindOwner, g, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ancestorOwner := range ancestorOwnerLists {
		ownerGrants := ancestorOwner.GetOwnerGrants()
		addGrants(ownerGrants.Roles, ownerGrants.Traits)
	}

	slices.Sort(grants.Roles)
	grants.Roles = slices.Compact(grants.Roles)

	for k, v := range grants.Traits {
		slices.Sort(v)
		grants.Traits[k] = slices.Compact(v)
	}

	return &grants, nil
}
