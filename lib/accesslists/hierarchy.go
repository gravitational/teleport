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

type MembersAndLocksGetter interface {
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}

// HierarchyNode represents an access list and its relationships.
type HierarchyNode struct {
	// AccessList is the Underlying AccessList object.
	AccessList *accesslist.AccessList
	// MemberUsers are users who are direct members of this list.
	MemberUsers map[string]*accesslist.AccessListMember
	// MemberLists are AccessLists that are direct members of this list.
	MemberLists map[string]*HierarchyNode
	// OwnerUsers are users who are direct Owners of this list.
	OwnerUsers map[string]*accesslist.Owner
	// OwnerLists are AccessLists that are direct Owners of this list.
	OwnerLists map[string]*HierarchyNode
	// MemberParents are AccessLists that have this list as a member.
	MemberParents map[string]*HierarchyNode
	// OwnerParents are AccessLists that have this list as an owner.
	OwnerParents map[string]*HierarchyNode
}

type hierarchy struct {
	Nodes map[string]*HierarchyNode
	Locks services.LockGetter
	Clock clockwork.Clock
}

// Hierarchy represents an interface for interacting with AccessLists and nested AccessLists through a tree-like structure.
type Hierarchy interface {
	// ValidateAccessListWithMembers validates the addition of a new or existing AccessList with a list of AccessListMembers.
	ValidateAccessListWithMembers(accessList *accesslist.AccessList, members []*accesslist.AccessListMember) error
	// ValidateAccessListOwner validates the addition of an existing AccessList as an Owner to another existing AccessList.
	ValidateAccessListOwner(parentListName string, owner *accesslist.Owner) error
	// ValidateAccessListMember validates the addition of an AccessListMember to an existing AccessList.
	ValidateAccessListMember(parentListName string, member *accesslist.AccessListMember) error
	// GetOwners returns a flattened list of Owners for an Access List, including inherited Owners.
	//
	// Returned Owners are not validated for requirements – use IsAccessListOwner
	// to validate an Owner's ownership status.
	GetOwners(accessListName string) ([]*accesslist.Owner, error)
	// GetMembers recursively fetches all non-list members for an AccessList.
	//
	// Returned Members are not validated for expiration or other requirements - use IsAccessListMember
	// to validate a Member's membership status.
	GetMembers(accessListName string) ([]*accesslist.AccessListMember, error)
	// IsAccessListOwner determines if a User is a valid Owner of an existing or new AccessList,
	// including via inheritance. If User has any inForce Locks, it will return an error.
	IsAccessListOwner(ctx context.Context, user types.User, accessListName string) (MembershipOrOwnershipType, error)
	// IsAccessListMember determines if a User is a valid Member of an existing AccessList,
	// including via inheritance. If User has any inForce Locks, it will return an error.
	IsAccessListMember(ctx context.Context, user types.User, accessListName string) (MembershipOrOwnershipType, error)
	// GetOwnerParents returns Access Lists where the given Access List is an owner.
	GetOwnerParents(accessListName string) ([]*accesslist.AccessList, error)
	// GetMemberParents returns Access Lists where the given Access List is a member.
	GetMemberParents(accessListName string) ([]*accesslist.AccessList, error)
	// GetInheritedGrants returns the combined Grants for an Access List's members, inherited from any ancestor lists.
	GetInheritedGrants(accessListName string) (*accesslist.Grants, error)
}

type HierarchyConfig struct {
	AccessLists []*accesslist.AccessList
	Members     MembersAndLocksGetter
	Locks       services.LockGetter
	Clock       clockwork.Clock
}

func checkAndSetDefaults(cfg *HierarchyConfig) error {
	if cfg.AccessLists == nil {
		cfg.AccessLists = []*accesslist.AccessList{}
	}
	if cfg.Members == nil {
		return trace.BadParameter("MembersGetter is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewHierarchy creates a new tree-like structure of AccessLists and their relationships, including Members and Owners.
// It validates the relationships between AccessLists and ensures that no cyclic references are created and that the depth of a
// branch does not exceed the maximum allowed depth.
//
// It returns a Hierarchy interface, useful for querying the validity of Membership and Ownership changes, and for determining
// a User's Membership or Ownership status for an AccessList, including inherited relationships.
func NewHierarchy(ctx context.Context, cfg HierarchyConfig) (Hierarchy, error) {
	err := checkAndSetDefaults(&cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &hierarchy{
		Nodes: make(map[string]*HierarchyNode),
		Locks: cfg.Locks,
		Clock: cfg.Clock,
	}

	for _, al := range cfg.AccessLists {
		h.Nodes[al.GetName()] = &HierarchyNode{
			AccessList:    al,
			MemberUsers:   make(map[string]*accesslist.AccessListMember),
			MemberLists:   make(map[string]*HierarchyNode),
			OwnerUsers:    make(map[string]*accesslist.Owner),
			OwnerLists:    make(map[string]*HierarchyNode),
			MemberParents: make(map[string]*HierarchyNode),
			OwnerParents:  make(map[string]*HierarchyNode),
		}
	}

	// Avoid non-deterministic order of processing here by iterating over the AccessLists instead of the Nodes map.
	for _, al := range cfg.AccessLists {
		node := h.Nodes[al.GetName()]

		for _, owner := range al.Spec.Owners {
			if owner.MembershipKind != accesslist.MembershipKindList {
				node.OwnerUsers[owner.Name] = &owner
				continue
			}
			ownerNode, exists := h.Nodes[owner.Name]
			// If the owner AccessList doesn't exist, and continue.
			if !exists {
				continue
			}
			if err := validateAddition(node, ownerNode, RelationshipKindOwner); err != nil {
				return nil, trace.Wrap(err)
			}
			node.OwnerLists[owner.Name] = ownerNode
			ownerNode.OwnerParents[node.AccessList.GetName()] = node
		}

		members, err := fetchMembers(ctx, al.GetName(), cfg.Members)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, member := range members {
			if member.Spec.MembershipKind != accesslist.MembershipKindList {
				node.MemberUsers[member.Spec.Name] = member
				continue
			}
			memberNode, exists := h.Nodes[member.Spec.Name]
			// If the member AccessList doesn't exist, continue.
			if !exists {
				continue
			}
			if err := validateAddition(node, memberNode, RelationshipKindMember); err != nil {
				return nil, trace.Wrap(err)
			}
			node.MemberLists[member.Spec.Name] = memberNode
			memberNode.MemberParents[node.AccessList.GetName()] = node
		}
	}

	return h, nil
}

func fetchMembers(ctx context.Context, accessListName string, membersGetter MembersAndLocksGetter) ([]*accesslist.AccessListMember, error) {
	var allMembers []*accesslist.AccessListMember
	pageToken := ""
	for {
		members, nextToken, err := membersGetter.ListAccessListMembers(ctx, accessListName, 0, pageToken)
		if err != nil {
			// If the AccessList doesn't exist yet, should return an empty list of members
			if trace.IsNotFound(err) {
				break
			}
			return nil, trace.Wrap(err)
		}
		allMembers = append(allMembers, members...)
		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}
	return allMembers, nil
}

// GetMembers recursively collects all non-list members for an AccessList.
func (h *hierarchy) GetMembers(accessListName string) ([]*accesslist.AccessListMember, error) {
	node, exists := h.Nodes[accessListName]
	if !exists {
		return nil, trace.NotFound("Access List '%s' not found", accessListName)
	}

	var allMembers []*accesslist.AccessListMember

	for _, member := range node.MemberUsers {
		allMembers = append(allMembers, member)
	}

	for _, memberList := range node.MemberLists {
		members, err := h.GetMembers(memberList.AccessList.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allMembers = append(allMembers, members...)
	}

	return allMembers, nil
}

func validateAddition(parentNode, childNode *HierarchyNode, kind RelationshipKind) error {
	kindStr := "a Member"
	if kind == RelationshipKindOwner {
		kindStr = "an Owner"
	}
	if isReachable(childNode, parentNode.AccessList.GetName(), make(map[string]struct{})) {
		return trace.BadParameter("Access List '%s' can't be added as %s of '%s' because '%s' is already included as a Member or Owner in '%s'", childNode.AccessList.Spec.Title, kindStr, parentNode.AccessList.Spec.Title, parentNode.AccessList.Spec.Title, childNode.AccessList.Spec.Title)
	}
	if exceedsMaxDepth(parentNode, childNode, kind) {
		return trace.BadParameter("Access List '%s' can't be added as %s of '%s' because it would exceed the maximum nesting depth of %d", childNode.AccessList.Spec.Title, kindStr, parentNode.AccessList.Spec.Title, accesslist.MaxAllowedDepth)
	}
	return nil
}

func isReachable(node *HierarchyNode, targetName string, visited map[string]struct{}) bool {
	if node.AccessList.GetName() == targetName {
		return true
	}
	if _, ok := visited[node.AccessList.GetName()]; ok {
		return false
	}
	visited[node.AccessList.GetName()] = struct{}{}

	// Traverse member lists
	for _, child := range node.MemberLists {
		if isReachable(child, targetName, visited) {
			return true
		}
	}
	// Traverse owner lists
	for _, owner := range node.OwnerLists {
		if isReachable(owner, targetName, visited) {
			return true
		}
	}
	return false
}

func exceedsMaxDepth(parentNode, childNode *HierarchyNode, kind RelationshipKind) bool {
	switch kind {
	case RelationshipKindOwner:
		// For Owners, only consider the depth downwards from the child node, since Ownership is not inherited Owners->Owners->Owners... as Membership is.
		return maxDepthDownwards(childNode, make(map[string]struct{})) > accesslist.MaxAllowedDepth
	default:
		// For Members, consider the depth upwards from the parent node, downwards from the child node, and the edge between them
		return maxDepthUpwards(parentNode, make(map[string]struct{}))+maxDepthDownwards(childNode, make(map[string]struct{}))+1 > accesslist.MaxAllowedDepth
	}
}

func maxDepthDownwards(node *HierarchyNode, seen map[string]struct{}) int {
	if _, ok := seen[node.AccessList.GetName()]; ok {
		return 0
	}
	seen[node.AccessList.GetName()] = struct{}{}
	maxDepth := 0
	for _, childNode := range node.MemberLists {
		// Depth is the max depth of all children, +1 for the edge to the child.
		depth := maxDepthDownwards(childNode, seen) + 1
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	delete(seen, node.AccessList.GetName())
	return maxDepth
}

func maxDepthUpwards(node *HierarchyNode, seen map[string]struct{}) int {
	if _, ok := seen[node.AccessList.GetName()]; ok {
		return 0
	}
	seen[node.AccessList.GetName()] = struct{}{}
	maxDepth := 0
	for _, parentNode := range node.MemberParents {
		// Depth upwards is the max depth of all parents, +1 for the edge to the parent.
		depth := maxDepthUpwards(parentNode, seen) + 1
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	delete(seen, node.AccessList.GetName())
	return maxDepth
}

// ValidateAccessListMember validates the addition of an AccessListMember to an existing AccessList.
func (h *hierarchy) ValidateAccessListMember(parentListName string, member *accesslist.AccessListMember) error {
	if member.Spec.MembershipKind != accesslist.MembershipKindList {
		return nil
	}
	return h.validateAccessListMemberOrOwner(parentListName, member.Spec.Name, RelationshipKindMember)
}

// ValidateAccessListOwner validates the addition of an existing AccessList as an Owner to another existing AccessList.
func (h *hierarchy) ValidateAccessListOwner(parentListName string, owner *accesslist.Owner) error {
	if owner.MembershipKind != accesslist.MembershipKindList {
		return nil
	}
	return h.validateAccessListMemberOrOwner(parentListName, owner.Name, RelationshipKindOwner)
}

func (h *hierarchy) validateAccessListMemberOrOwner(parentListName string, memberOrOwnerName string, kind RelationshipKind) error {
	parentNode, parentExists := h.Nodes[parentListName]
	if !parentExists {
		return trace.NotFound("Access List '%s' not found", parentListName)
	}
	memberOrOwnerNode, memberOrOwnerExists := h.Nodes[memberOrOwnerName]
	if !memberOrOwnerExists {
		return trace.NotFound("Access List '%s' not found", memberOrOwnerName)
	}
	if err := validateAddition(parentNode, memberOrOwnerNode, kind); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ValidateAccessListWithMembers validates the addition of a new or existing AccessList with a list of AccessListMembers.
func (h *hierarchy) ValidateAccessListWithMembers(accessList *accesslist.AccessList, members []*accesslist.AccessListMember) error {
	var tempNode *HierarchyNode
	existingNode, exists := h.Nodes[accessList.GetName()]
	if exists {
		// Reuse existing node's reverse pointers to parents.
		tempNode = &HierarchyNode{
			AccessList:    accessList,
			MemberUsers:   make(map[string]*accesslist.AccessListMember),
			MemberLists:   make(map[string]*HierarchyNode),
			OwnerUsers:    make(map[string]*accesslist.Owner),
			OwnerLists:    make(map[string]*HierarchyNode),
			MemberParents: existingNode.MemberParents,
			OwnerParents:  existingNode.OwnerParents,
		}
	} else {
		tempNode = &HierarchyNode{
			AccessList:    accessList,
			MemberUsers:   make(map[string]*accesslist.AccessListMember),
			MemberLists:   make(map[string]*HierarchyNode),
			OwnerUsers:    make(map[string]*accesslist.Owner),
			OwnerLists:    make(map[string]*HierarchyNode),
			MemberParents: make(map[string]*HierarchyNode),
			OwnerParents:  make(map[string]*HierarchyNode),
		}
	}

	for _, owner := range accessList.Spec.Owners {
		if owner.MembershipKind != accesslist.MembershipKindList {
			tempNode.OwnerUsers[owner.Name] = &owner
			continue
		}
		ownerNode, ownerExists := h.Nodes[owner.Name]
		if !ownerExists {
			return trace.NotFound("Owner Access List '%s' not found", owner.Name)
		}
		if err := validateAddition(tempNode, ownerNode, RelationshipKindOwner); err != nil {
			return trace.Wrap(err)
		}
		tempNode.OwnerLists[owner.Name] = ownerNode
	}
	for _, member := range members {
		if member.Spec.MembershipKind != accesslist.MembershipKindList {
			tempNode.MemberUsers[member.Spec.Name] = member
			continue
		}
		memberNode, memberExists := h.Nodes[member.Spec.Name]
		if !memberExists {
			return trace.NotFound("Member Access List '%s' not found", member.Spec.Name)
		}
		if err := validateAddition(tempNode, memberNode, RelationshipKindMember); err != nil {
			return trace.Wrap(err)
		}
		tempNode.MemberLists[member.Spec.Name] = memberNode
	}
	return nil
}

// GetOwners returns a flattened list of Owners for an Access List, including inherited Owners.
//
// Returned Owners are not validated for expiration or other requirements – use IsAccessListOwner
// to validate an Owner's ownership status.
func (h *hierarchy) GetOwners(accessListName string) ([]*accesslist.Owner, error) {
	node, exists := h.Nodes[accessListName]
	if !exists {
		return nil, trace.NotFound("Access List '%s' not found", accessListName)
	}

	visited := make(map[string]struct{})
	owners := h.collectOwners(node, visited, []*accesslist.Owner{})

	return owners, nil
}

// IsAccessListOwner determines if a User is a valid Owner of an existing or new AccessList,
// including via inheritance. If User has any inForce Locks, it will return an error.
func (h *hierarchy) IsAccessListOwner(ctx context.Context, user types.User, accessListName string) (MembershipOrOwnershipType, error) {
	// Allow for Locks to be nil when not provided in constructor.
	if h.Locks != nil {
		locks, err := h.Locks.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return MembershipOrOwnershipTypeNone, trace.Wrap(err)
		}

		if len(locks) > 0 {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' is currently locked", user.GetName())
		}
	}

	node, exists := h.Nodes[accessListName]
	if !exists {
		return MembershipOrOwnershipTypeNone, trace.NotFound("Access List '%s' not found", accessListName)
	}

	// Check explicit owners
	if _, ok := node.OwnerUsers[user.GetName()]; ok {
		// Verify ownership requirements using provided AccessList
		if !UserMeetsRequirements(user, node.AccessList.Spec.OwnershipRequires) {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", user.GetName(), node.AccessList.Spec.Title)
		}
		return MembershipOrOwnershipTypeExplicit, nil
	}

	// Check inherited ownership
	visited := make(map[string]struct{})
	isOwner, err := h.isInheritedOwner(ctx, user, node, visited)
	if err != nil {
		return MembershipOrOwnershipTypeNone, trace.Wrap(err)
	}
	if isOwner {
		// ALso needs to meet ownership requirements of the parent.
		if !UserMeetsRequirements(user, node.AccessList.Spec.OwnershipRequires) {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", user.GetName(), node.AccessList.Spec.Title)
		}
		return MembershipOrOwnershipTypeInherited, nil
	}
	return MembershipOrOwnershipTypeNone, nil
}

func (h *hierarchy) isInheritedOwner(ctx context.Context, user types.User, node *HierarchyNode, visited map[string]struct{}) (bool, error) {
	if _, ok := visited[node.AccessList.GetName()]; ok {
		return false, nil
	}
	visited[node.AccessList.GetName()] = struct{}{}
	for _, ownerList := range node.OwnerLists {
		// Check if identity is a member of ownerList
		memberType, err := h.IsAccessListMember(ctx, user, ownerList.AccessList.GetName())
		if err != nil {
			return false, trace.Wrap(err)
		}
		if memberType != MembershipOrOwnershipTypeNone {
			return true, nil
		}
		// Recurse into ownerList's owners
		isOwner, err := h.isInheritedOwner(ctx, user, ownerList, visited)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if isOwner {
			return true, nil
		}
	}
	return false, nil
}

// IsAccessListMember determines if a User is a valid Member of an existing AccessList,
// including via inheritance. If User has any inForce Locks, it will return an error.
func (h *hierarchy) IsAccessListMember(ctx context.Context, user types.User, accessListName string) (MembershipOrOwnershipType, error) {
	// Allow for Locks to be nil when not provided in constructor.
	if h.Locks != nil {
		locks, err := h.Locks.GetLocks(ctx, true, types.LockTarget{
			User: user.GetName(),
		})
		if err != nil {
			return MembershipOrOwnershipTypeNone, trace.Wrap(err)
		}

		if len(locks) > 0 {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' is currently locked", user.GetName())
		}
	}

	node, exists := h.Nodes[accessListName]
	if !exists {
		return MembershipOrOwnershipTypeNone, trace.NotFound("Access List '%s' not found", accessListName)
	}
	// Check explicit members
	if _, ok := node.MemberUsers[user.GetName()]; ok {
		// Verify membership requirements
		if !UserMeetsRequirements(user, node.AccessList.Spec.MembershipRequires) {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", user.GetName(), node.AccessList.Spec.Title)
		}
		// Verify membership is not expired
		if !node.MemberUsers[user.GetName()].Spec.Expires.IsZero() && !h.Clock.Now().Before(node.MemberUsers[user.GetName()].Spec.Expires) {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), node.AccessList.Spec.Title)
		}
		return MembershipOrOwnershipTypeExplicit, nil
	}
	// Check inherited membership
	visited := make(map[string]struct{})
	isMember, expired := h.isInheritedMember(user, node, visited)
	if expired {
		return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s's membership in Access List '%s' has expired", user.GetName(), node.AccessList.Spec.Title)
	}
	if isMember {
		// Also needs to meet membership requirements of the parent.
		if !UserMeetsRequirements(user, node.AccessList.Spec.MembershipRequires) {
			return MembershipOrOwnershipTypeNone, trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", user.GetName(), node.AccessList.Spec.Title)
		}
		return MembershipOrOwnershipTypeInherited, nil
	}
	return MembershipOrOwnershipTypeNone, nil
}

func (h *hierarchy) isInheritedMember(user types.User, node *HierarchyNode, visited map[string]struct{}) (bool, bool) {
	if _, ok := visited[node.AccessList.GetName()]; ok {
		return false, false
	}
	visited[node.AccessList.GetName()] = struct{}{}
	expired := false
	for _, memberList := range node.MemberLists {
		// Check if identity is a member of memberList
		if member, ok := memberList.MemberUsers[user.GetName()]; ok {
			// Check if membership is expired
			if !member.Spec.Expires.IsZero() && !h.Clock.Now().Before(member.Spec.Expires) {
				expired = true
				// Avoid non-deterministic behavior here: If user's membership is expired, then
				// continue checking, in case their membership in a related list is still valid.
				continue
			} else {
				expired = false
			}
			// Verify membership requirements
			if !UserMeetsRequirements(user, memberList.AccessList.Spec.MembershipRequires) {
				continue
			}
			return true, false
		}
		// Recurse into memberList's members
		isMember, expiredRecurse := h.isInheritedMember(user, memberList, visited)
		if expiredRecurse && !expired {
			expired = true
		}
		if isMember {
			return true, false
		}
	}
	return false, expired
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

func (h *hierarchy) collectOwners(node *HierarchyNode, visited map[string]struct{}, owners []*accesslist.Owner) []*accesslist.Owner {
	if _, ok := visited[node.AccessList.GetName()]; ok {
		return owners
	}
	visited[node.AccessList.GetName()] = struct{}{}

	// Collect direct owner users
	for _, owner := range node.OwnerUsers {
		owners = append(owners, owner)
	}

	// For owner lists, we need to collect their members as owners
	for _, ownerList := range node.OwnerLists {
		// Collect members from owner lists as potential owners
		memberVisited := make(map[string]struct{})
		owners = h.collectMembersAsOwners(ownerList, memberVisited, owners)
	}

	return owners
}

func (h *hierarchy) collectMembersAsOwners(node *HierarchyNode, visited map[string]struct{}, owners []*accesslist.Owner) []*accesslist.Owner {
	if _, ok := visited[node.AccessList.GetName()]; ok {
		return owners
	}
	visited[node.AccessList.GetName()] = struct{}{}

	// Collect direct member users as owners
	for _, member := range node.MemberUsers {
		owners = append(owners, &accesslist.Owner{
			Name:             member.Spec.Name,
			Description:      member.Metadata.Description,
			IneligibleStatus: "",
			MembershipKind:   accesslist.MembershipKindUser,
		})
	}

	// Recursively collect members from member lists
	for _, memberList := range node.MemberLists {
		owners = h.collectMembersAsOwners(memberList, visited, owners)
	}

	return owners
}

func (h *hierarchy) getAncestorsFor(accessListName string, kind RelationshipKind) ([]*accesslist.AccessList, error) {
	node, exists := h.Nodes[accessListName]
	if !exists {
		return nil, trace.NotFound("Access List '%s' not found", accessListName)
	}
	visited := make(map[string]struct{})
	ancestorsMap := make(map[string]*accesslist.AccessList)
	h.collectAncestors(node, kind, visited, ancestorsMap)
	ancestors := make([]*accesslist.AccessList, 0, len(ancestorsMap))
	for _, al := range ancestorsMap {
		ancestors = append(ancestors, al)
	}
	return ancestors, nil
}

func (h *hierarchy) collectAncestors(node *HierarchyNode, kind RelationshipKind, visited map[string]struct{}, ancestors map[string]*accesslist.AccessList) {
	if _, ok := visited[node.AccessList.GetName()]; ok {
		return
	}
	visited[node.AccessList.GetName()] = struct{}{}

	switch kind {
	case RelationshipKindOwner:
		// Add direct owner parents to ancestors
		for _, ownerParent := range node.OwnerParents {
			ancestors[ownerParent.AccessList.GetName()] = ownerParent.AccessList
		}
		// Recursively traverse member parents
		for _, memberParent := range node.MemberParents {
			h.collectAncestors(memberParent, kind, visited, ancestors)
		}
	default:
		// Only collect and add member parents
		for _, memberParent := range node.MemberParents {
			ancestors[memberParent.AccessList.GetName()] = memberParent.AccessList
			h.collectAncestors(memberParent, kind, visited, ancestors)
		}
	}
}

// GetInheritedGrants returns the combined Grants for an Access List's members, inherited from any ancestor lists.
func (h *hierarchy) GetInheritedGrants(accessListName string) (*accesslist.Grants, error) {
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
	ancestorLists, err := h.getAncestorsFor(accessListName, RelationshipKindMember)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ancestor := range ancestorLists {
		memberGrants := ancestor.GetGrants()
		addGrants(memberGrants.Roles, memberGrants.Traits)
	}

	// Get ancestors via owner relationship
	ancestorOwnerLists, err := h.getAncestorsFor(accessListName, RelationshipKindOwner)
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

// GetMemberParents returns Access Lists where the given Access List is a direct Member.
func (h *hierarchy) GetMemberParents(accessListName string) ([]*accesslist.AccessList, error) {
	node, exists := h.Nodes[accessListName]
	if !exists {
		return nil, trace.NotFound("Access List '%s' not found", accessListName)
	}

	var parentAccessLists []*accesslist.AccessList
	for _, parentNode := range node.MemberParents {
		parentAccessLists = append(parentAccessLists, parentNode.AccessList)
	}
	return parentAccessLists, nil
}

// GetOwnerParents returns Access Lists where the given Access List is a direct Owner.
func (h *hierarchy) GetOwnerParents(accessListName string) ([]*accesslist.AccessList, error) {
	node, exists := h.Nodes[accessListName]
	if !exists {
		return nil, trace.NotFound("Access List '%s' not found", accessListName)
	}

	var parentAccessLists []*accesslist.AccessList
	for _, parentNode := range node.OwnerParents {
		parentAccessLists = append(parentAccessLists, parentNode.AccessList)
	}
	return parentAccessLists, nil
}
