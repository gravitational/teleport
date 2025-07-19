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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
)

// ValidateAccessListWithMembers validates AccessList with a list of its members. If the
// existingAccessList is non-nil it also checks if this is a valid update transition.
func ValidateAccessListWithMembers(ctx context.Context, existingAccessList, accessList *accesslist.AccessList, members []*accesslist.AccessListMember, g AccessListAndMembersGetter) error {
	if err := validateAccessListUpdate(existingAccessList, accessList); err != nil {
		return trace.Wrap(err)
	}
	if err := validateAccessListNesting(ctx, accessList, members, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateAccessListUpdate checks if the AccessList update is valid. In particular it verifies
// that immutable fields are not changed. It does nothing if the existingAccessList is nil.
func validateAccessListUpdate(existingAccessList, accessList *accesslist.AccessList) error {
	if existingAccessList == nil {
		return nil
	}
	if !accessList.Spec.Type.Equals(existingAccessList.Spec.Type) {
		return trace.BadParameter("access_list %q type %q cannot be changed to %q",
			accessList.Metadata.Name, existingAccessList.Spec.Type, accessList.Spec.Type)
	}
	return nil
}

// validateAccessListNesting validates if nested AccessList owners and members meet max depth (of
// 10) requirement and don't create cycles.
func validateAccessListNesting(ctx context.Context, accessList *accesslist.AccessList, members []*accesslist.AccessListMember, g AccessListAndMembersGetter) error {
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
		memberList, err := g.GetAccessList(ctx, member.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		if err := validateAddition(ctx, accessList, memberList, RelationshipKindMember, g); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
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
	return validateAccessListMemberOrOwner(ctx, parentList, member.GetName(), RelationshipKindMember, g)
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
			childList, err := g.GetAccessList(ctx, member.GetName())
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
