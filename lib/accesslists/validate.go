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

// ValidateAccessListWithMembers makes sure the given AccessList and it's members is valid before
// storing it. If the existingAccessList is non-nil it also checks if this is a valid update
// transition. It takes into account validation of the nested access lists membership.
func ValidateAccessListWithMembers(ctx context.Context, existingAccessList, accessList *accesslist.AccessList, members []*accesslist.AccessListMember, g AccessListAndMembersGetter) error {
	if err := validateAccessList(accessList); err != nil {
		return trace.Wrap(err)
	}
	if err := validateAccessListUpdate(existingAccessList, accessList); err != nil {
		return trace.Wrap(err)
	}
	if err := validateAccessListNesting(ctx, accessList, members, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateAccessList makes sure the given AccessList is valid before storing in the backend.
func validateAccessList(a *accesslist.AccessList) error {
	if err := validateType(a.Spec.Type); err != nil {
		return trace.Wrap(err)
	}

	if len(a.Spec.Owners) == 0 {
		return trace.BadParameter("owners are missing")
	}

	if a.IsReviewable() || a.Spec.Audit.Recurrence.Frequency != 0 {
		switch a.Spec.Audit.Recurrence.Frequency {
		case accesslist.OneMonth, accesslist.ThreeMonths, accesslist.SixMonths, accesslist.OneYear:
		default:
			return trace.BadParameter("audit recurrence frequency is an invalid value")
		}
	}

	if a.IsReviewable() || a.Spec.Audit.Recurrence.DayOfMonth != 0 {
		switch a.Spec.Audit.Recurrence.DayOfMonth {
		case accesslist.FirstDayOfMonth, accesslist.FifteenthDayOfMonth, accesslist.LastDayOfMonth:
		default:
			return trace.BadParameter("audit recurrence day of month is an invalid value")
		}
	}

	if a.IsReviewable() {
		if a.Spec.Audit.NextAuditDate.IsZero() {
			return trace.BadParameter("next audit date is not set")

		}

		if a.Spec.Audit.Notifications.Start == 0 {
			return trace.BadParameter("audit notifications start is not set")
		}
	}

	return nil
}

// validateType validates if access list type is a known value. It deliberately excludes
// [accesslist.DeprecatedDynamic] as it should be converted to [accesslist.Default] in the
// [accesslist.AccessList.CheckAndSetDefaults].
func validateType(t accesslist.Type) error {
	switch t {
	case accesslist.Default, accesslist.Static, accesslist.SCIM:
		return nil
	default:
		return trace.BadParameter("unknown access list type %q", t)
	}
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
		if err := validateAccessListOwner(ctx, accessList, owner, g); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, member := range members {
		if err := ValidateAccessListMember(ctx, accessList, member, g); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ValidateAccessListMember validates AccessListMember. That includes nested AccessList members
// validation.
func ValidateAccessListMember(
	ctx context.Context,
	parentList *accesslist.AccessList,
	member *accesslist.AccessListMember,
	g AccessListAndMembersGetter,
) error {
	if err := validateAccessListMemberBasic(member); err != nil {
		return trace.Wrap(err)
	}
	if err := validateAccessListMemberOrOwnerNesting(ctx, parentList, member.GetName(), RelationshipKindMember, member.Spec.MembershipKind, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateAccessListMemberBasic performs basic fields validation for AccessListMember.
func validateAccessListMemberBasic(member *accesslist.AccessListMember) error {
	if member.Spec.AccessList == "" {
		return trace.BadParameter("member %s: access_list field empty", member.Metadata.Name)
	}
	if member.Spec.Name != member.Metadata.Name {
		return trace.BadParameter("member metadata name = %q and spec name = %q must be equal", member.Metadata.Name, member.Spec.Name)
	}
	if member.Spec.Joined.IsZero() || member.Spec.Joined.Unix() == 0 {
		return trace.BadParameter("member %s: joined field empty or missing", member.Metadata.Name)
	}
	if member.Spec.AddedBy == "" {
		return trace.BadParameter("member %s: added_by field is empty", member.Metadata.Name)
	}
	return nil
}

// validateAccessListOwner Owner for an AccessList. That includes nested AccessList owners
// validation.
func validateAccessListOwner(
	ctx context.Context,
	parentList *accesslist.AccessList,
	owner accesslist.Owner,
	g AccessListAndMembersGetter,
) error {
	if err := validateAccessListMemberOrOwnerNesting(ctx, parentList, owner.Name, RelationshipKindOwner, owner.MembershipKind, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateAccessListMemberOrOwnerNesting(
	ctx context.Context,
	parentList *accesslist.AccessList,
	memberOrOwnerName string,
	relationshipKind RelationshipKind,
	membershipKind string,
	g AccessListAndMembersGetter,
) error {
	if membershipKind != accesslist.MembershipKindList {
		return nil
	}
	// If it is a AccessList member/owner then the referenced AccessList must exist.
	memberOrOwnerList, err := g.GetAccessList(ctx, memberOrOwnerName)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateAddition(ctx, parentList, memberOrOwnerList, relationshipKind, g); err != nil {
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
