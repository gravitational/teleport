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
	"errors"
	"maps"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
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

// AccessListAndMembersGetter is a minimal interface for fetching AccessLists by name, and AccessListMembers for an Access List.
type AccessListAndMembersGetter interface {
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
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
		childMembers, err := getMembersFor(ctx, member.GetName(), g, visited)
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

// collectOwners is a helper to recursively collect all Owners for an Access List, including inherited Owners.
func collectOwners(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter, owners map[string]*accesslist.Owner, visited map[string]struct{}) error {
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
			Name:             member.GetName(),
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
			childListName := member.GetName()
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

// userLockedError is used to check specific condition of user being locked with [IsUserLocked]. It
// is also being matched by [trace.IsAccessDenied] while allowing creating a dynamic error message
// containing the user name.
type userLockedError struct{ err error }

// newUserLockedError returns a new userLockedError.
func newUserLockedError(user string) userLockedError {
	return userLockedError{trace.AccessDenied("User %q is currently locked", user)}
}

func (e userLockedError) Unwrap() error { return e.err }
func (e userLockedError) Error() string { return e.err.Error() }

// IsUserLocked checks if the error was a result of the Access List member user having a lock.
func IsUserLocked(err error) bool {
	return errors.As(err, &userLockedError{})
}

// IsAccessListOwner checks if the given user is the Access List owner. It returns an error matched
// by [IsUserLocked] if the user is locked.
func IsAccessListOwner(
	ctx context.Context,
	user types.User,
	accessList *accesslist.AccessList,
	g AccessListAndMembersGetter,
	lockGetter services.LockGetter,
	clock clockwork.Clock,
) (accesslistv1.AccessListUserAssignmentType, error) {
	if lockGetter != nil {
		locks, err := lockGetter.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, trace.Wrap(err)
		}
		if len(locks) > 0 {
			return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, newUserLockedError(user.GetName())
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
			return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, nil
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
			if membershipType != accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED {
				if !UserMeetsRequirements(user, accessList.Spec.OwnershipRequires) {
					ownershipErr = trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
					continue
				}
				return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, nil
			}
		}
	}

	return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, trace.Wrap(ownershipErr)
}

// IsAccessListMember checks if the given user is the Access List member. It returns an error
// matched by [IsUserLocked] if the user is locked.
func IsAccessListMember(
	ctx context.Context,
	user types.User,
	accessList *accesslist.AccessList,
	g AccessListAndMembersGetter,
	lockGetter services.LockGetter,
	clock clockwork.Clock,
) (accesslistv1.AccessListUserAssignmentType, error) {
	if lockGetter != nil {
		locks, err := lockGetter.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, trace.Wrap(err)
		}
		if len(locks) > 0 {
			return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, newUserLockedError(user.GetName())
		}
	}

	members, err := fetchMembers(ctx, accessList.GetName(), g)
	if err != nil {
		return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, trace.Wrap(err)
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
			return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, nil
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
			if membershipType != accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED {
				if !UserMeetsRequirements(user, accessList.Spec.MembershipRequires) {
					membershipErr = trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
					continue
				}
				if !member.Spec.Expires.IsZero() && !clock.Now().Before(member.Spec.Expires) {
					membershipErr = trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), accessList.Spec.Title)
					continue
				}
				return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, nil
			}
		}
	}

	return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, trace.Wrap(membershipErr)
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

type ancestorOptions struct {
	validateUserRequirement bool
	clock                   clockwork.Clock
	user                    types.User
}

func (o *ancestorOptions) validate() error {
	if o.validateUserRequirement {
		if o.user == nil {
			return trace.BadParameter("user is required when validateUserRequirement is true")
		}
		if o.clock == nil {
			o.clock = clockwork.NewRealClock()
		}
	}
	return nil
}

// ancestorOption is a functional option for configuring the behavior of GetAncestorsFor.
type ancestorOption func(*ancestorOptions)

func withUserRequirementsCheck(user types.User, clock clockwork.Clock) ancestorOption {
	return func(opts *ancestorOptions) {
		opts.validateUserRequirement = true
		opts.user = user
		opts.clock = clock
	}
}

// HierarchyConfig holds dependencies for building access list hierarchies.
type HierarchyConfig struct {
	// AccessListService is used to fetch Access Lists and their members.
	AccessListsService AccessListAndMembersGetter
	// Getter is used to fetch Access Lists and their members.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *HierarchyConfig) CheckAndSetDefaults() error {
	if c.AccessListsService == nil {
		return trace.BadParameter("AccessListsService is required")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// Hierarchy provides methods to compute access list hierarchies.
type Hierarchy struct {
	HierarchyConfig
}

// NewHierarchy constructs a HierarchyService with the given config.
func NewHierarchy(cfg HierarchyConfig) (*Hierarchy, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Hierarchy{
		HierarchyConfig: cfg,
	}, nil
}

// GetHierarchyForUser builds the hierarchy of Access Lists for a given user,
// starting from the provided list and traversing upward through its ancestors
// using MemberOf/OwnerOf relationships. The returned hierarchy includes the
// starting list (if the user meets the requirements) and may include ancestor
// lists where the user satisfies membership or ownership requirements.
// If the user fails to meet the requirements at any point, that branch is excluded.
func (s *Hierarchy) GetHierarchyForUser(ctx context.Context, accessList *accesslist.AccessList, user types.User) (memberHierarchy, ownerHierarchy []*accesslist.AccessList, err error) {
	if s.validDirectOwner(user, accessList) {
		// User Is direct owner and meet the ownership requirements
		// Include access list from the owner hierarchy
		// and check if there is more ownership via nested ownership
		// or via chained membership -> ownership relationships
		// e.g. A has nested ownership B, B has nested membership C, C has direct members like alice
		ownerHierarchy = append(ownerHierarchy, accessList)
	}
	ok, err := s.validMembership(ctx, accessList, user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !ok {
		// User is not a valid member of the starting access list
		// return current owner hierarchy (not empty if user has a direct ownership)
		// and empty member hierarchy
		return memberHierarchy, ownerHierarchy, nil
	}
	// User is a valid member of the starting access list
	// Include access list in the member hierarchy
	memberHierarchy = append(memberHierarchy, accessList)

	// Fetch ancestors via MemberOf edges while checking user requirements
	ancestors, err := getAncestors(ctx, accessList, RelationshipKindMember, s.AccessListsService, withUserRequirementsCheck(user, s.Clock))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	owners, err := s.expandOwnerOf(ctx, ancestors, user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return append(memberHierarchy, ancestors...), append(ownerHierarchy, owners...), nil
}

func (s *Hierarchy) validMembership(ctx context.Context, list *accesslist.AccessList, user types.User) (bool, error) {
	m, err := s.AccessListsService.GetAccessListMember(ctx, list.GetName(), user.GetName())
	if err != nil {
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	if m.IsExpired(s.Clock.Now()) || !UserMeetsRequirements(user, list.GetMembershipRequires()) {
		return false, nil
	}
	return true, nil
}

// Expand ancestors via OwnerOf edges while checking user requirements
func (s *Hierarchy) expandOwnerOf(ctx context.Context, ancestors []*accesslist.AccessList, user types.User) ([]*accesslist.AccessList, error) {
	var out []*accesslist.AccessList
	for _, v := range ancestors {
		for _, name := range v.Status.OwnerOf {
			lst, err := s.AccessListsService.GetAccessList(ctx, name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if UserMeetsRequirements(user, lst.GetOwnershipRequires()) {
				out = append(out, lst)
			}
		}
	}
	return out, nil
}

func (s *Hierarchy) validDirectOwner(user types.User, acl *accesslist.AccessList) bool {
	if !UserMeetsRequirements(user, acl.GetOwnershipRequires()) {
		return false
	}
	for _, v := range acl.Spec.Owners {
		if v.Name == user.GetName() {
			return true
		}
	}
	return false
}

// GetAncestorsFor calculates and returns the set of Ancestor ACLs depending on
// the supplied relationship criteria. Order of the ancestor list is undefined.
func GetAncestorsFor(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, g AccessListAndMembersGetter) ([]*accesslist.AccessList, error) {
	return getAncestors(ctx, accessList, kind, g)
}

func getAncestors(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, g AccessListAndMembersGetter, opts ...ancestorOption) ([]*accesslist.AccessList, trace.Error) {
	ancestorsMap := make(map[string]*accesslist.AccessList)
	if err := collectAncestors(ctx, accessList, kind, g, make(map[string]struct{}), ancestorsMap, opts...); err != nil {
		return nil, trace.Wrap(err)
	}
	ancestors := slices.Collect(maps.Values(ancestorsMap))
	return ancestors, nil
}

func collectAncestors(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, g AccessListAndMembersGetter, visited map[string]struct{}, ancestors map[string]*accesslist.AccessList, opts ...ancestorOption) error {
	options := &ancestorOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if err := options.validate(); err != nil {
		return trace.Wrap(err)
	}
	if _, ok := visited[accessList.GetName()]; ok {
		return nil
	}
	visited[accessList.GetName()] = struct{}{}

	isDirectMembershipExpired := func(acl, member string) (bool, error) {
		if !options.validateUserRequirement {
			return false, nil
		}
		m, err := g.GetAccessListMember(ctx, acl, member)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return m.IsExpired(options.clock.Now()), nil
	}

	userMeetsRequirements := func(r accesslist.Requires) bool {
		if options.user == nil {
			return true
		}
		return UserMeetsRequirements(options.user, r)
	}

	if kind == RelationshipKindOwner {
		// Add parents where this list is an owner to ancestors
		for _, ownerParent := range accessList.Status.OwnerOf {
			ownerParentAcl, err := g.GetAccessList(ctx, ownerParent)
			if err != nil {
				return trace.Wrap(err)
			}
			if !userMeetsRequirements(ownerParentAcl.Spec.OwnershipRequires) {
				continue
			}
			ancestors[ownerParent] = ownerParentAcl
		}
	}
	for _, memberParent := range accessList.Status.MemberOf {
		memberParentAcl, err := g.GetAccessList(ctx, memberParent)
		if err != nil {
			return trace.Wrap(err)
		}
		expired, err := isDirectMembershipExpired(memberParent, accessList.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		if expired || !userMeetsRequirements(memberParentAcl.Spec.MembershipRequires) {
			continue
		}

		if kind == RelationshipKindMember {
			ancestors[memberParent] = memberParentAcl
		}
		if err := collectAncestors(ctx, memberParentAcl, kind, g, visited, ancestors, opts...); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetInheritedMembershipRequires returns the combined Requires for an Access List's members,
// inherited from any ancestor lists, and the Access List's own MembershipRequires.
func GetInheritedMembershipRequires(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter) (*accesslist.Requires, error) {
	ownRequires := accessList.GetMembershipRequires()
	ancestors, err := GetAncestorsFor(ctx, accessList, RelationshipKindMember, g)
	if err != nil {
		return &ownRequires, trace.Wrap(err)
	}

	roles := ownRequires.Roles
	traits := ownRequires.Traits

	for _, ancestor := range ancestors {
		requires := ancestor.GetMembershipRequires()
		roles = append(roles, requires.Roles...)
		for traitKey, traitValues := range requires.Traits {
			if _, exists := traits[traitKey]; !exists {
				traits[traitKey] = []string{}
			}
			traits[traitKey] = append(traits[traitKey], traitValues...)
		}
	}

	slices.Sort(roles)
	roles = slices.Compact(roles)

	for k, v := range traits {
		slices.Sort(v)
		traits[k] = slices.Compact(v)
	}

	return &accesslist.Requires{
		Roles:  roles,
		Traits: traits,
	}, nil
}

// GetInheritedGrants returns the combined Grants for an Access List's members, inherited from any ancestor lists.
func GetInheritedGrants(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter) (*accesslist.Grants, error) {
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
	ancestorLists, err := GetAncestorsFor(ctx, accessList, RelationshipKindMember, g)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ancestor := range ancestorLists {
		memberGrants := ancestor.GetGrants()
		addGrants(memberGrants.Roles, memberGrants.Traits)
	}

	// Get ancestors via owner relationship
	ancestorOwnerLists, err := GetAncestorsFor(ctx, accessList, RelationshipKindOwner, g)
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
