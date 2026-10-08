// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package delegation

import (
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/set"
)

// AllDelegatableRoles filters the given role set down to only roles that may be
// delegated, by applying `allow.delegation.roles` and `deny.delegation.roles`.
//
// If any roles are unsafe to "drop" (i.e. they contain `deny` conditions), but
// the user is not permitted to delegate them, this method will return an error
// as there is no safe way to proceed.
//
// You should call this method with the user's *standing* roles, not temporary
// roles assigned by access requests or access lists, to prevent them from using
// delegation to extend their access beyond the allowed time.
func AllDelegatableRoles(roleSet []types.Role) ([]types.Role, error) {
	var (
		allowStatements      = set.New[string]()
		denyStatements       = set.New[string]()
		undroppableRoleNames = set.New[string]()
		rolesByName          = make(map[string]types.Role)
	)
	for _, role := range roleSet {
		rolesByName[role.GetName()] = role

		allowStatements.Add(role.GetDelegationRoles(types.Allow)...)
		denyStatements.Add(role.GetDelegationRoles(types.Deny)...)

		if roleIsUndroppable(role) {
			undroppableRoleNames.Add(role.GetName())
		}
	}

	// Start with all of the roles the user holds.
	allowedRoleNames := roleNames(roleSet)

	// Filter it down to any explicitly allowed roles, unless there's a wildcard.
	if !allowStatements.Contains(types.Wildcard) {
		allowedRoleNames.Intersection(allowStatements)
	}

	// If there's a deny wildcard, empty the list entirely. Otherwise, filter
	// out explicitly denied roles.
	if denyStatements.Contains(types.Wildcard) {
		allowedRoleNames = set.New[string]()
	} else {
		allowedRoleNames.Subtract(denyStatements)
	}

	// Check if there are any undroppable roles that the user is not allowed to
	// delegate. This should be a fairly unusual edge-case, the user themselves
	// cannot fix it and must speak to their cluster administrator to fix their
	// permissions.
	unsafeDrops := undroppableRoleNames.Subtract(allowedRoleNames).Elements()
	if len(unsafeDrops) > 0 {
		sort.Strings(unsafeDrops)
		return nil, trace.AccessDenied(
			"You are assigned the following roles that must be delegated, but you do not have permission to do so: [%s]",
			strings.Join(unsafeDrops, ", "),
		)
	}

	// Collect them back up into a RoleSet.
	finalRoleSet := make([]types.Role, len(allowedRoleNames))
	for idx, roleName := range allowedRoleNames.Elements() {
		finalRoleSet[idx] = rolesByName[roleName]
	}
	return finalRoleSet, nil
}

// CheckDelegatedRoles checks whether the requested roles may be delegated by
// applying `allow.delegation.roles` and `deny.delegation.roles` - and ensuring
// any "undroppable" roles (i.e. those containing `deny` conditions) are included.
//
// You should call this method with the user's *standing* roles, not temporary
// roles assigned by access requests or access lists, to prevent them from using
// delegation to extend their access beyond the allowed time.
func CheckDelegatedRoles(roleSet []types.Role, delegatedRoleNames []string) error {
	roles, err := AllDelegatableRoles(roleSet)
	if err != nil {
		return trace.Wrap(err)
	}

	requested := set.New(delegatedRoleNames...)
	allowed := roleNames(roles)

	// Check if there are any undroppable roles that the user isn't delegating.
	var unsafeDrops []string
	for _, role := range roles {
		if requested.Contains(role.GetName()) {
			continue
		}
		if roleIsUndroppable(role) {
			unsafeDrops = append(unsafeDrops, role.GetName())
		}
	}
	if len(unsafeDrops) > 0 {
		// We have already checked the user is actually allowed to delegate these
		// roles in AllDelegatableRoles.
		sort.Strings(unsafeDrops)
		return trace.AccessDenied(
			"You must delegate the following roles: [%s]",
			strings.Join(unsafeDrops, ", "),
		)
	}

	missing := requested.Subtract(allowed).Elements()
	if len(missing) > 0 {
		sort.Strings(missing)
		return trace.AccessDenied(
			"You do not have permission to delegate the following roles: [%s]",
			strings.Join(missing, ", "),
		)
	}

	return nil
}

// roleIsUndroppable determines whether the role is unsafe to drop because it
// contains any `deny` conditions.
func roleIsUndroppable(role types.Role) bool {
	return !role.GetRoleConditions(types.Deny).IsEmpty()
}

func roleNames(roles []types.Role) set.Set[string] {
	names := set.New[string]()
	for _, role := range roles {
		names.Add(role.GetName())
	}
	return names
}
