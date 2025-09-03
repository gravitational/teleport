/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"maps"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// RelationshipKind represents the type of relationship: member or owner.
type RelationshipKind int

const (
	RelationshipKindMember RelationshipKind = iota
	RelationshipKindOwner
)

var (
	userAssignUnspecified = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED
	userAssignExplicit    = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT
	userAssignInherited   = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED
)

// AccessListAndMembersGetter is a minimal interface for fetching AccessLists by name, and AccessListMembers for an Access List.
type AccessListAndMembersGetter interface {
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
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
	// LockService is used to fetch user locks.
	LockService services.LockGetter
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

// GetMembersFor returns a flattened list of Members for an Access List, including inherited Members.
//
// Returned Members are not validated for expiration or other requirements – use IsAccessListMember
// to validate a Member's membership status.
func (s *Hierarchy) GetMembersFor(ctx context.Context, accessListName string) ([]*accesslist.AccessListMember, error) {
	return s.getMembersFor(ctx, accessListName, make(map[string]struct{}))
}

func (s *Hierarchy) getMembersFor(ctx context.Context, accessListName string, visited map[string]struct{}) ([]*accesslist.AccessListMember, error) {
	if _, ok := visited[accessListName]; ok {
		return nil, nil
	}
	visited[accessListName] = struct{}{}

	members, err := s.fetchMembers(ctx, accessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var allMembers []*accesslist.AccessListMember
	for _, member := range members {
		if member.Spec.MembershipKind != accesslist.MembershipKindList {
			allMembers = append(allMembers, member)
			continue
		}
		childMembers, err := s.getMembersFor(ctx, member.GetName(), visited)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allMembers = append(allMembers, childMembers...)
	}

	return allMembers, nil
}

func (s *Hierarchy) fetchMembers(ctx context.Context, accessListName string) ([]*accesslist.AccessListMember, error) {
	return fetchMembers(ctx, accessListName, s.AccessListsService)
}

// fetchMembers is a simple helper to fetch all top-level AccessListMembers for an AccessList.
func fetchMembers(ctx context.Context, accessListName string, g AccessListAndMembersGetter) ([]*accesslist.AccessListMember, error) {
	out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, i int, s string) ([]*accesslist.AccessListMember, string, error) {
		return g.ListAccessListMembers(ctx, accessListName, i, s)
	}))
	if err != nil {
		if trace.IsNotFound(err) {
			// If the AccessList doesn't exist yet, should return an empty list of members
			return []*accesslist.AccessListMember{}, nil
		}
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// collectOwners is a helper to recursively collect all Owners for an Access List, including inherited Owners.
func (s *Hierarchy) collectOwners(ctx context.Context, accessList *accesslist.AccessList, owners map[string]*accesslist.Owner, visited map[string]struct{}) error {
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
		ownerMembers, err := s.collectMembersAsOwners(ctx, owner.Name, visited)
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
func (s *Hierarchy) collectMembersAsOwners(ctx context.Context, accessListName string, visited map[string]struct{}) ([]*accesslist.Owner, error) {
	owners := make([]*accesslist.Owner, 0)
	if _, ok := visited[accessListName]; ok {
		return owners, nil
	}
	visited[accessListName] = struct{}{}

	listMembers, err := s.GetMembersFor(ctx, accessListName)
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
func (s *Hierarchy) GetOwnersFor(ctx context.Context, accessList *accesslist.AccessList) ([]*accesslist.Owner, error) {
	ownersMap := make(map[string]*accesslist.Owner)
	if err := s.collectOwners(ctx, accessList, ownersMap, make(map[string]struct{})); err != nil {
		return nil, trace.Wrap(err)
	}
	owners := make([]*accesslist.Owner, 0, len(ownersMap))
	for _, owner := range ownersMap {
		owners = append(owners, owner)
	}
	return owners, nil
}

func (s *Hierarchy) userIsLocked(ctx context.Context, user types.User) error {
	if s.LockService != nil {
		locks, err := s.LockService.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(locks) > 0 {
			return newUserLockedError(user.GetName())
		}
	}
	return nil
}

// IsAccessListOwner checks if the given user is the Access List owner. It returns an error matched
// by [IsUserLocked] if the user is locked.
func (s *Hierarchy) IsAccessListOwner(ctx context.Context, user types.User, accessList *accesslist.AccessList) (accesslistv1.AccessListUserAssignmentType, error) {
	if err := s.userIsLocked(ctx, user); err != nil {
		return userAssignUnspecified, trace.Wrap(err)
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
			return userAssignExplicit, nil
		}
		// Is user an inherited owner through any potential owner AccessLists?
		if owner.MembershipKind == accesslist.MembershipKindList {
			ownerAccessList, err := s.AccessListsService.GetAccessList(ctx, owner.Name)
			if err != nil {
				ownershipErr = trace.Wrap(err)
				continue
			}
			// Since we already verified that the user is not locked, don't provide lockGetter here
			membershipType, err := s.IsAccessListMember(ctx, user, ownerAccessList)
			if err != nil {
				ownershipErr = trace.Wrap(err)
				continue
			}
			if membershipType != userAssignUnspecified {
				if !UserMeetsRequirements(user, accessList.Spec.OwnershipRequires) {
					ownershipErr = trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
					continue
				}
				return userAssignInherited, nil
			}
		}
	}

	return userAssignUnspecified, trace.Wrap(ownershipErr)
}

// IsAccessListMember checks if the given user is the Access List member. It returns an error
// matched by [IsUserLocked] if the user is locked.
func (s *Hierarchy) IsAccessListMember(ctx context.Context, user types.User, accessList *accesslist.AccessList) (accesslistv1.AccessListUserAssignmentType, error) {
	if err := s.userIsLocked(ctx, user); err != nil {
		return userAssignUnspecified, trace.Wrap(err)
	}

	members, err := fetchMembers(ctx, accessList.GetName(), s.AccessListsService)
	if err != nil {
		return userAssignUnspecified, trace.Wrap(err)
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
			if member.IsExpired(s.Clock.Now()) {
				membershipErr = trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), accessList.Spec.Title)
				continue
			}
			return userAssignExplicit, nil
		}
		// Is user an inherited member through any potential member AccessLists?
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			memberAccessList, err := s.AccessListsService.GetAccessList(ctx, member.GetName())
			if err != nil {
				membershipErr = trace.Wrap(err)
				continue
			}
			// Since we already verified that the user is not locked, don't provide lockGetter here
			membershipType, err := s.IsAccessListMember(ctx, user, memberAccessList)
			if err != nil {
				membershipErr = trace.Wrap(err)
				continue
			}
			if membershipType != userAssignUnspecified {
				if !UserMeetsRequirements(user, accessList.Spec.MembershipRequires) {
					membershipErr = trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", user.GetName(), accessList.Spec.Title)
					continue
				}
				if member.IsExpired(s.Clock.Now()) {
					membershipErr = trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), accessList.Spec.Title)
					continue
				}
				return userAssignInherited, nil
			}
		}
	}

	return userAssignUnspecified, trace.Wrap(membershipErr)
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
	ancestors, err := s.GetAncestorsFor(ctx, accessList, RelationshipKindMember, withUserRequirementsCheck(user, s.Clock))
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
func (s *Hierarchy) GetAncestorsFor(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, opts ...ancestorOption) ([]*accesslist.AccessList, trace.Error) {
	ancestorsMap := make(map[string]*accesslist.AccessList)
	if err := s.collectAncestors(ctx, accessList, kind, make(map[string]struct{}), ancestorsMap, opts...); err != nil {
		return nil, trace.Wrap(err)
	}
	ancestors := slices.Collect(maps.Values(ancestorsMap))
	return ancestors, nil
}

func (s *Hierarchy) collectAncestors(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, visited map[string]struct{}, ancestors map[string]*accesslist.AccessList, opts ...ancestorOption) error {
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
		m, err := s.AccessListsService.GetAccessListMember(ctx, acl, member)
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
			ownerParentAcl, err := s.AccessListsService.GetAccessList(ctx, ownerParent)
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
		memberParentAcl, err := s.AccessListsService.GetAccessList(ctx, memberParent)
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
		if err := s.collectAncestors(ctx, memberParentAcl, kind, visited, ancestors, opts...); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
