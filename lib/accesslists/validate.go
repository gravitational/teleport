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
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

var (
	// ErrDeniedAccessListDeletion is returned when an Access List which also also a
	// member of another Access List is attempted for deletion.
	ErrDeniedAccessListDeletion = &trace.AccessDeniedError{Message: "Access List with nested Access List membership cannot be deleted"}
	// ErrCyclicMembership is returned when a cyclic Access List membership
	// is detected. E.g. List A is a member of List B and List B is a member
	// of List A.
	ErrCyclicMembership = &trace.BadParameterError{Message: "cyclic membership not allowed"}
	// ErrMaxNestedMembershipDepth is returned when Access List membership
	// exceeds maximum supported nested membership depth. Default max depth
	// is defined in accesslist.MaxAllowedDepth.
	ErrMaxNestedMembershipDepth = &trace.BadParameterError{Message: "excdeeds maximum nested Access List membership depth"}
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

	// Any access lists that assign scoped roles cannot contain
	// membership_requires or ownership_requires.
	hasRequirements := !a.Spec.MembershipRequires.IsEmpty() || !a.Spec.OwnershipRequires.IsEmpty()
	hasScopedRoleGrants := len(a.Spec.Grants.ScopedRoles) > 0 || len(a.Spec.OwnerGrants.ScopedRoles) > 0
	if hasScopedRoleGrants && hasRequirements {
		return trace.BadParameter("access lists cannot contain both scoped_role grants and non-empty membership_requires or ownership_requires blocks")
	}

	if a.Scope != "" {
		if err := scopes.StrongValidate(a.Scope); err != nil {
			return trace.Wrap(err, "access list has invalid scope")
		}

		// Scoped access lists cannot contain requirements.
		if hasRequirements {
			return trace.BadParameter("scoped access lists cannot contain non-empty membership_requires or ownership_requires blocks")
		}

		// Scoped access lists cannot grant unscoped roles or traits.
		if len(a.Spec.Grants.Roles) > 0 || len(a.Spec.OwnerGrants.Roles) > 0 {
			return trace.BadParameter("scoped access lists cannot grant unscoped roles")
		}
		if len(a.Spec.Grants.Traits) > 0 || len(a.Spec.OwnerGrants.Traits) > 0 {
			return trace.BadParameter("scoped access lists cannot grant traits")
		}

		// TODO(nklaassen): after scoped role assignments have been updated to
		// refer to scoped access lists with scope-qualified names, access
		// lists should do the same. At that point we must validate that the
		// scope of origin of each granted scoped role is equal or ancestor to
		// the scope of the access list.
	}

	if err := validateScopedRoleGrants(a); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func validateScopedRoleGrants(a *accesslist.AccessList) error {
	uniqueScopedRoleGrants := make(map[accesslist.ScopedRoleGrant]struct{})
	validateScopedRoleGrant := func(grant accesslist.ScopedRoleGrant) error {
		if _, alreadyValidated := uniqueScopedRoleGrants[grant]; alreadyValidated {
			return nil
		}
		switch {
		case grant.Role == "":
			return trace.BadParameter("role is empty")
		case grant.Scope == "":
			return trace.BadParameter("scope is empty")
		}

		if err := scopes.StrongValidate(grant.Scope); err != nil {
			return trace.Wrap(err, "validating scope")
		}

		if scopes.Compare(grant.Scope, scopes.Root) == scopes.Equivalent {
			return trace.BadParameter("root scope cannot be used as a scope of effect")
		}

		if a.Scope != "" {
			// Scoped access lists can only assign scoped roles to their own scope or a descendent scope.
			if !scopes.ScopeOfOrigin(a.Scope).IsAssignableToScopeOfEffect(grant.Scope) {
				return trace.BadParameter("scoped role grant has scope %q that is not a sub-scope of the access list's scope %q", grant.Scope, a.Scope)
			}
		}

		uniqueScopedRoleGrants[grant] = struct{}{}
		return nil
	}
	for i, grant := range a.Spec.Grants.ScopedRoles {
		if err := validateScopedRoleGrant(grant); err != nil {
			return trace.Wrap(err, "validating grants.scoped_roles[%d]", i)
		}
	}
	for i, grant := range a.Spec.OwnerGrants.ScopedRoles {
		if err := validateScopedRoleGrant(grant); err != nil {
			return trace.Wrap(err, "validating owner_grants.scoped_roles[%d]", i)
		}
	}
	// Unique scoped role grants per access list have the same limit as role
	// grants per scoped role assignment, because each access list materializes
	// to a single scoped roles assignment.
	if len(uniqueScopedRoleGrants) > scopedaccess.MaxRolesPerAssignment {
		return trace.BadParameter("access list contains too many unique scoped role grants (max %d)", scopedaccess.MaxRolesPerAssignment)
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
	if accessList.Scope != "" && len(members) > 0 {
		// TODO(nklaassen): support scoped access list members.
		return trace.BadParameter("scoped access list members are not yet supported")
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
	if err := validateAccessListMemberBasic(parentList, member); err != nil {
		return trace.Wrap(err)
	}
	memberName, err := MemberScopeQualifiedName(member)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateAccessListMemberOrOwnerNesting(ctx, parentList, memberName, RelationshipKindMember, member.Spec.MembershipKind, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateAccessListMemberBasic performs basic fields validation for AccessListMember
// and performs the cross membership integrity check.
func validateAccessListMemberBasic(parent *accesslist.AccessList, member *accesslist.AccessListMember) error {
	if member.Scope != "" {
		// TODO(nklaassen): support scoped access list members.
		return trace.BadParameter("scoped access list members are not yet supported")
	}
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
	// The member must belong to the parent access list.
	if member.Spec.AccessList != parent.GetName() {
		return trace.BadParameter("member %s: spec.access_list field %q doesn't match parent list name %q", member.Metadata.Name, member.Spec.AccessList, parent.GetName())
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
	ownerName, err := OwnerScopeQualifiedName(owner)
	if err != nil {
		return trace.Wrap(err)
	}
	if ownerName.Scope != "" {
		if err := ownerName.ToScopesQualifiedName().StrongValidate(); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := validateAccessListMemberOrOwnerNesting(ctx, parentList, ownerName, RelationshipKindOwner, owner.MembershipKind, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateAccessListMemberOrOwnerNesting(
	ctx context.Context,
	parentList *accesslist.AccessList,
	memberOrOwnerName NormalizedSQN,
	relationshipKind RelationshipKind,
	membershipKind string,
	g AccessListAndMembersGetter,
) error {
	if !accesslist.IsMembershipKindList(membershipKind) {
		return nil
	}

	if err := validateMemberOrOwnerScopeHierarchy(parentList, memberOrOwnerName, relationshipKind); err != nil {
		return trace.Wrap(err)
	}

	// If it is a AccessList member/owner then the referenced AccessList must exist.
	memberOrOwnerList, err := getAccessList(ctx, g, memberOrOwnerName)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateAddition(ctx, parentList, memberOrOwnerList, relationshipKind, g); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateMemberOrOwnerScopeHierarchy(
	list *accesslist.AccessList,
	memberOrOwnerName NormalizedSQN,
	relationshipKind RelationshipKind,
) error {
	if memberOrOwnerName.Scope == "" {
		// Unscoped access lists can be members or owners of scoped or unscoped lists.
		return nil
	}

	if list.GetScope() == memberOrOwnerName.Scope {
		// Members or owners are always allowed at the same scope of the parent/owned list.
		return nil
	}

	kindStr := "a Member"
	if relationshipKind == RelationshipKindOwner {
		kindStr = "an Owner"
	}

	if list.GetScope() == "" {
		return trace.BadParameter("Access List '%s' cannot name '%s' as %s because scoped access lists cannot be members or owners of unscoped access lists",
			list.Spec.Title, memberOrOwnerName.String(), kindStr)
	}

	if !scopes.PolicyResourceScope(list.GetScope()).CanDependOnStateFromPolicyResourceAtScope(memberOrOwnerName.Scope) {
		return trace.BadParameter("Access List '%s' cannot name '%s' as %s because it is not at an equal or ancestor scope",
			list.Spec.Title, memberOrOwnerName.String(), kindStr)
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
	reachable, err := isReachable(ctx, childList, parentList, make(map[NormalizedSQN]struct{}), g)
	if err != nil {
		return trace.Wrap(err)
	}
	if reachable {
		return trace.Wrap(ErrCyclicMembership, "Access List '%s' can't be added as %s of '%s' because '%s' is already included as a Member or Owner in '%s'",
			childList.Spec.Title, kindStr, parentList.Spec.Title, parentList.Spec.Title, childList.Spec.Title)
	}

	// Max depth check
	exceeds, err := exceedsMaxDepth(ctx, parentList, childList, kind, g)
	if err != nil {
		return trace.Wrap(err)
	}
	if exceeds {
		return trace.Wrap(ErrMaxNestedMembershipDepth, "Access List '%s' can't be added as %s of '%s' because it would exceed the maximum nesting depth of %d",
			childList.Spec.Title, kindStr, parentList.Spec.Title, accesslist.MaxAllowedDepth)
	}

	return nil
}

func isReachable(
	ctx context.Context,
	currentList *accesslist.AccessList,
	targetList *accesslist.AccessList,
	visited map[NormalizedSQN]struct{},
	g AccessListAndMembersGetter,
) (bool, error) {
	if ScopeQualifiedName(currentList) == ScopeQualifiedName(targetList) {
		return true, nil
	}
	if _, ok := visited[ScopeQualifiedName(currentList)]; ok {
		return false, nil
	}
	visited[ScopeQualifiedName(currentList)] = struct{}{}

	// Traverse member lists
	listMembers, err := fetchMembers(ctx, ScopeQualifiedName(currentList), g)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, member := range listMembers {
		if !member.IsList() {
			continue
		}
		childListName, err := MemberScopeQualifiedName(member)
		if err != nil {
			return false, trace.Wrap(err)
		}
		childList, err := getAccessList(ctx, g, childListName)
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

	// Traverse owner lists
	for _, owner := range currentList.Spec.Owners {
		if !owner.IsMembershipKindList() {
			continue
		}
		ownerName, err := OwnerScopeQualifiedName(owner)
		if err != nil {
			return false, trace.Wrap(err)
		}
		ownerList, err := getAccessList(ctx, g, ownerName)
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
		depthDownwards, err := maxDepthDownwards(ctx, ScopeQualifiedName(childList), make(map[NormalizedSQN]struct{}), g)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return depthDownwards > accesslist.MaxAllowedDepth, nil
	default:
		// For Members, consider the depth upwards from the parent node, downwards from the child node, and the edge between them
		depthUpwards, err := maxDepthUpwards(ctx, parentList, make(map[NormalizedSQN]struct{}), g)
		if err != nil {
			return false, trace.Wrap(err)
		}
		depthDownwards, err := maxDepthDownwards(ctx, ScopeQualifiedName(childList), make(map[NormalizedSQN]struct{}), g)
		if err != nil {
			return false, trace.Wrap(err)
		}
		totalDepth := depthUpwards + depthDownwards + 1 // +1 for the edge between parent and child
		return totalDepth > accesslist.MaxAllowedDepth, nil
	}
}
