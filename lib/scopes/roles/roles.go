// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package roles

import (
	"iter"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

const (
	// MaxRolesPerAssignment is the maximum number of roles@scope assignments that a given scoped role assignment
	// resource may contain. This value is so low because our backend limits the number of keys that can be associated
	// with a single atomic operation. Any significant increase to this value would necessitate a change to the
	// scoped role backend model.
	MaxRolesPerAssignment = 16

	// KindScopedRole is the kind of a scoped role resource.
	KindScopedRole = "scoped_role"

	// KindScopedRoleAssignment is the kind of a scoped role assignment resource.
	KindScopedRoleAssignment = "scoped_role_assignment"

	// maxAssignableScopes is the maximum number of assignable scopes that a given scoped role resource may contain. Note that
	// unlike MaxRolesPerAssignment, this is a fairly arbitrary limit and there isn't a strong reason to keep it low other than
	// to avoid excess resource size and to keep our options open for the future.
	maxAssignableScopes = 16
)

// RoleIsAssignableAtScope checks if the given role is assignable at the given scope.
func RoleIsAssignableAtScope(role *accessv1.ScopedRole, scope string) bool {
	for assignableScope := range WeakValidatedAssignableScopes(role) {
		if scopes.Glob(assignableScope).Matches(scope) {
			return true
		}
	}

	return false
}

// WeakValidatedAssignableScopes is a helper for iterating all well formed assignable scopes for a given role.
func WeakValidatedAssignableScopes(role *accessv1.ScopedRole) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, assignableScope := range role.GetSpec().GetAssignableScopes() {
			if err := scopes.WeakValidateGlob(assignableScope); err != nil {
				// ignore invalid assignable scopes
				continue
			}

			if !scopes.Glob(assignableScope).IsSubjectToPolicyResourceScope(role.GetScope()) {
				// ignore assignable scopes that do not conform to assignment subjugation rules
				continue
			}

			if !yield(assignableScope) {
				return
			}
		}
	}
}

// WeakValidateRole valides a role to ensure it is free of obvious issues that would render it unusable and/or
// induce serious unintended behavior. Prefer using this function for validating roles loaded from "internal" sources
// (e.g. backend/control-plane), and [StrongValidateRole] for validating roles loaded from "external" sources (e.g. user input).
func WeakValidateRole(role *accessv1.ScopedRole) error {
	if err := commonValidateRole(role); err != nil {
		return trace.Wrap(err)
	}

	if err := scopes.WeakValidate(role.GetScope()); err != nil {
		return trace.BadParameter("scoped role %q has invalid scope: %v", role.GetMetadata().GetName(), err)
	}

	// NOTE: in strong validation, this is where we'd check that the assignable scopes are valid. In weak validation
	// we don't do that and instead rely on invalid assignable scopes being filtered out
	// and excluded during runtime assignment validation checks. This helps us ensure that outdated agents continue
	// to be able to understand and process the subset of assignments that they are able to reason about.

	return nil
}

// StrongValidateRole performs robust validation of a role to ensure it complies with all expected constraints. Prefer
// using this function for validating roles loaded from "external" sources (e.g. user input), and [WeakValidateRole] for
// validating roles loaded from "internal" sources (e.g. backend/control-plane).
func StrongValidateRole(role *accessv1.ScopedRole) error {
	if err := commonValidateRole(role); err != nil {
		return trace.Wrap(err)
	}

	if err := validateRoleName(role.GetMetadata().GetName()); err != nil {
		return trace.BadParameter("scoped role name %q does not conform to segment naming rules: %v", role.GetMetadata().GetName(), err)
	}

	if err := scopes.StrongValidate(role.GetScope()); err != nil {
		return trace.BadParameter("scoped role %q has invalid scope: %v", role.GetMetadata().GetName(), err)
	}

	if len(role.GetSpec().GetAssignableScopes()) == 0 {
		return trace.BadParameter("scoped role %q does not have any assignable scopes", role.GetMetadata().GetName())
	}

	if len(role.GetSpec().GetAssignableScopes()) > maxAssignableScopes {
		return trace.BadParameter("scoped role %q has too many assignable scopes (max %d)", role.GetMetadata().GetName(), maxAssignableScopes)
	}

	for _, scopeGlob := range role.GetSpec().GetAssignableScopes() {
		if err := scopes.StrongValidateGlob(scopeGlob); err != nil {
			return trace.BadParameter("scoped role %q has invalid assignable scope %q: %v", role.GetMetadata().GetName(), scopeGlob, err)
		}

		if !scopes.Glob(scopeGlob).IsSubjectToPolicyResourceScope(role.GetScope()) {
			return trace.BadParameter("scoped role %q has assignable scope %q that is not a sub-scope of the role's scope %q", role.GetMetadata().GetName(), scopeGlob, role.GetScope())
		}
	}

	return nil
}

func validateRoleName(name string) error {
	// note: having the scope name be validated as a segment name is a bit of an arbitrary choice, but its basically
	// equivalent to what we would want from a standalone name requirement, and there may even be some future benefit
	// if we ever need to encode a role assignment as a scope-like name.
	return trace.Wrap(scopes.StrongValidateSegment(name))
}

// commonValidateRole performs the subset of role validation common to both weak and strong validation.
func commonValidateRole(role *accessv1.ScopedRole) error {
	if role.GetMetadata().GetName() == "" {
		return trace.BadParameter("scoped role is missing metadata.name")
	}

	if role.GetKind() == "" {
		return trace.BadParameter("scoped role %q is missing kind", role.GetMetadata().GetName())
	}

	if role.GetKind() != KindScopedRole {
		return trace.BadParameter("scoped role %q has invalid kind %q, expected %q", role.GetMetadata().GetName(), role.GetKind(), KindScopedRole)
	}

	if role.GetSubKind() != "" {
		return trace.BadParameter("scoped role %q has unknown sub_kind %q", role.GetMetadata().GetName(), role.GetSubKind())
	}

	if role.GetVersion() == "" {
		return trace.BadParameter("scoped role %q is missing version", role.GetMetadata().GetName())
	}

	if role.GetVersion() != types.V1 {
		return trace.BadParameter("scoped role %q has unsupported version %q (expected %q)", role.GetMetadata().GetName(), role.GetVersion(), types.V1)
	}

	if role.GetScope() == "" {
		return trace.BadParameter("scoped role %q is missing scope", role.GetMetadata().GetName())
	}

	return nil
}

// WeakValidatedSubAssignments is a helper for iterating all well formed sub-assignments within a given assignment. Note that the concept
// of a well-formed sub-assignment is distinct from wether or not an assignment is "boken/invalidated" in the sense used when
// deciding wether or not an access-control check can be performed for a given scope. The only thing that is being filtered out
// by this iterator is sub-assignments that are so obviously misconfigured that we can't reason about them at all. Sub-assignments
// returned by this iterator may still be broken because they assign a nonexistent role, or to a scope that the target role is not
// assignable to.
func WeakValidatedSubAssignments(assignment *accessv1.ScopedRoleAssignment) iter.Seq[*accessv1.Assignment] {
	return func(yield func(*accessv1.Assignment) bool) {
		for _, subAssignment := range assignment.GetSpec().GetAssignments() {
			if subAssignment.GetRole() == "" {
				// ignore sub-assignments with missing role
				continue
			}

			if err := scopes.WeakValidate(subAssignment.GetScope()); err != nil {
				// ignore sub-assignments with invalid scopes
				continue
			}

			if !scopes.PolicyAssignmentScope(subAssignment.GetScope()).IsSubjectToPolicyResourceScope(assignment.GetScope()) {
				// ignore sub-assignments with scopes that do not conform to assignment subjugation rules
				continue
			}

			if !yield(subAssignment) {
				return
			}
		}
	}
}

// WeakValidateAssignment validates an assignment to ensure it is free of obvious issues that would render it unusable and/or
// induce serious unintended behavior. Prefer using this function for validating assignments loaded from "internal" sources
// (e.g. backend/control-plane), and [StrongValidateAssignment] for validating assignments loaded from "external" sources (e.g. user input).
func WeakValidateAssignment(assignment *accessv1.ScopedRoleAssignment) error {
	if err := commonValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	if err := scopes.WeakValidate(assignment.GetScope()); err != nil {
		return trace.BadParameter("scoped role assignment %q has invalid scope: %v", assignment.GetMetadata().GetName(), err)
	}

	// NOTE: in strong validation, this is where we'd check that the sub-assignments are valid. In weak validation
	// we don't do that and instead rely on invalid sub-assignments being filtered out and excluded during runtime
	// assignment resolution.

	return nil
}

// StrongValidateAssignment performs robust validation of an assignment to ensure it complies with all expected constraints. Prefer
// using this function for validating assignments loaded from "external" sources (e.g. user input), and [WeakValidateAssignment] for
// validating assignments loaded from "internal" sources (e.g. backend/control-plane).
func StrongValidateAssignment(assignment *accessv1.ScopedRoleAssignment) error {
	if err := commonValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	if _, err := uuid.Parse(assignment.GetMetadata().GetName()); err != nil {
		return trace.BadParameter("scoped role assignment %q has invalid name (must be uuid): %v", assignment.GetMetadata().GetName(), err)
	}

	if err := scopes.StrongValidate(assignment.GetScope()); err != nil {
		return trace.BadParameter("scoped role assignment %q has invalid scope: %v", assignment.GetMetadata().GetName(), err)
	}

	if len(assignment.GetSpec().GetAssignments()) == 0 {
		return trace.BadParameter("scoped role assignment %q does not assign any roles", assignment.GetMetadata().GetName())
	}

	if len(assignment.GetSpec().GetAssignments()) > MaxRolesPerAssignment {
		return trace.BadParameter("scoped role assignment %q contains too many sub-assignments (max %d)", assignment.GetMetadata().GetName(), MaxRolesPerAssignment)
	}

	for i, subAssignment := range assignment.GetSpec().GetAssignments() {
		if subAssignment.GetRole() == "" {
			return trace.BadParameter("scoped role assignment %q is missing role in sub-assignment %d", assignment.GetMetadata().GetName(), i)
		}

		if err := validateRoleName(subAssignment.GetRole()); err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid role name in sub-assignment %d: %v", assignment.GetMetadata().GetName(), i, err)
		}

		if err := scopes.StrongValidate(subAssignment.GetScope()); err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid scope in sub-assignment %d: %v", assignment.GetMetadata().GetName(), i, err)
		}

		if !scopes.PolicyAssignmentScope(subAssignment.GetScope()).IsSubjectToPolicyResourceScope(assignment.GetScope()) {
			return trace.BadParameter("scoped role assignment %q has sub-assignment %d with scope %q that is not a sub-scope of the assignment's scope %q", assignment.GetMetadata().GetName(), i, subAssignment.GetScope(), assignment.GetScope())
		}
	}

	return nil
}

func commonValidateAssignment(assignment *accessv1.ScopedRoleAssignment) error {
	if assignment.GetMetadata().GetName() == "" {
		return trace.BadParameter("scoped role assignment is missing metadata.name")
	}

	if assignment.GetKind() == "" {
		return trace.BadParameter("scoped role assignment %q is missing kind", assignment.GetMetadata().GetName())
	}

	if assignment.GetKind() != KindScopedRoleAssignment {
		return trace.BadParameter("scoped role assignment %q has invalid kind %q, expected %q", assignment.GetMetadata().GetName(), assignment.GetKind(), KindScopedRoleAssignment)
	}

	if assignment.GetSubKind() != "" {
		return trace.BadParameter("scoped role assignment %q has unknown sub_kind %q", assignment.GetMetadata().GetName(), assignment.GetSubKind())
	}

	if assignment.GetVersion() == "" {
		return trace.BadParameter("scoped role assignment %q is missing version", assignment.GetMetadata().GetName())
	}

	if assignment.GetVersion() != types.V1 {
		return trace.BadParameter("scoped role assignment %q has unsupported version %q (expected %q)", assignment.GetMetadata().GetName(), assignment.GetVersion(), types.V1)
	}

	if assignment.GetScope() == "" {
		return trace.BadParameter("scoped role assignment %q is missing scope", assignment.GetMetadata().GetName())
	}

	if assignment.GetSpec().GetUser() == "" {
		return trace.BadParameter("scoped role assignment %q is missing spec.user", assignment.GetMetadata().GetName())
	}

	return nil
}
