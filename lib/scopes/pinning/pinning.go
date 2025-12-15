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

package pinning

import (
	"iter"

	"github.com/gravitational/trace"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
)

// StrongValidate checks if the scope pin is well-formed according to all scope pin rules. This function
// *must* be used to validate any scope pin being created from scratch. Use of this function should be
// avoided for checking the validity of existing scope pins extracted from certificates and/or as part
// of any agent-side checks. Prefer using [WeakValidate] in those cases.
func StrongValidate(pin *scopesv1.Pin) error {
	if pin == nil {
		return trace.BadParameter("missing scope pin")
	}

	if err := scopes.StrongValidate(pin.GetScope()); err != nil {
		return trace.Errorf("invalid pinned scope: %w", err)
	}

	if len(pin.GetAssignments()) == 0 {
		// in theory there isn't any harm in allowing pins to be created without any assignments, but we're choosing to err
		// on the side of caution for now. this limitation may be lifted later.
		// NOTE: if lifting this restriction, the equivalent check in the pin building logic must also be lifted.
		return trace.BadParameter("scope pin at %q contains no assignments", pin.GetScope())
	}

	for scope, assignment := range pin.GetAssignments() {
		if err := scopes.StrongValidate(scope); err != nil {
			return trace.Errorf("invalid pinned assignment scope %q: %w", scope, err)
		}

		if len(assignment.GetRoles()) == 0 {
			return trace.BadParameter("scope pin at %q contains empty assignment for scope %q", pin.GetScope(), scope)
		}

		if !PinCompatibleWithPolicyScope(pin, scope) {
			return trace.BadParameter("scope pin at %q contains assignment(s) at incompatible scope %q", pin.GetScope(), scope)
		}
	}

	return nil
}

// WeakValidate performs a weak form of validation on a scope pin. This function is intended to catch
// bugs/incompatibilities that might have resulted in a scope pin too malformed for us to safely reason
// about (e.g. due to significant version drift). Use this function to validate scope pins extracted from
// certificates or otherwsie propagated from the control plane. Prefer using [StrongValidate] when building
// a new scope pin from scratch.
func WeakValidate(pin *scopesv1.Pin) error {
	if pin == nil {
		return trace.BadParameter("missing scope pin")
	}

	if err := scopes.WeakValidate(pin.GetScope()); err != nil {
		return trace.Errorf("invalid pinned scope: %w", err)
	}

	// validate that the scope of assignments are well-formed. due to how scoped access checks work, we cannot
	// perform any checks if any scopes are malformed as we cannot determine whether or not the assignments
	// within that scope ought to apply to the scope of the resource being targeted, and we cannot fallback
	// to the strategy of only evaluating parent assignments since we cannot safely determine the intended
	// parent/child relationship with the invalid scope.
	for scope := range pin.GetAssignments() {
		if err := scopes.WeakValidate(scope); err != nil {
			return trace.Errorf("invalid pinned assignment scope: %w", err)
		}
	}

	return nil
}

// PinCompatibleWithPolicyScope checks if a given policy scope might affect access subject to the pin. When building a
// pin, all compatible scoped role assignments must be encoded on the pin. Access while pinned may be affected by any policy
// that is ancestral, equivalent, or descendant to the pin scope, meaning that all non-orthogonal scopes are compatible.
func PinCompatibleWithPolicyScope(pin *scopesv1.Pin, scope string) bool {
	return scopes.Compare(pin.GetScope(), scope) != scopes.Orthogonal
}

// PinAppliesToResourceScope verifies that the given pin pins a scope that applies to the given resource scope. Resources
// that are not subject to the pinned scope cannot be accessed by the pinned identity, even if assigned roles would
// otherwise allow access. Note that this is conceptually distinct from whether or not we can perform *access checks* with
// a pin at a given scope. A pin at scope /foo/bar might still yield an allow decision at resource scope /foo, but only if
// the target resource is assigned to /foo/bar or one of its descendants.
func PinAppliesToResourceScope(pin *scopesv1.Pin, resourceScope string) bool {
	return scopes.PolicyScope(pin.GetScope()).AppliesToResourceScope(resourceScope)
}

// AssignmentsForResourceScope returns a sequence of pinned assignments relevant to the target resource scope, starting
// from the root scope and descending to the target. This is the correct order to evaluate access checks in, and is a suitable
// building block for access-checking logic.
func AssignmentsForResourceScope(pin *scopesv1.Pin, resourceScope string) (iter.Seq2[string, *scopesv1.PinnedAssignments], error) {
	if !PinAppliesToResourceScope(pin, resourceScope) {
		// a pin with a scope that does not apply to the resource scope should be caught at an
		// earlier stage, but failure to catch this may be a security issue, so we include a
		// redundant check here to prevent accidental misuse.
		return nil, trace.Errorf("invalid resource scope %q for scope pin at %q in assignment lookup (this is a bug)", resourceScope, pin.GetScope())
	}

	return AssignmentsForResourceScopeUnchecked(pin, resourceScope), nil
}

// AssignmentsForResourceScopeUnchecked is like AssignmentsForResourceScope, but does not perform any validation to ensure that the target
// resource scope is valid for the pin. This is used internally by some access-checker building logic which does its own validation
// of resource scoping.
func AssignmentsForResourceScopeUnchecked(pin *scopesv1.Pin, resourceScope string) iter.Seq2[string, *scopesv1.PinnedAssignments] {
	return func(yield func(string, *scopesv1.PinnedAssignments) bool) {
		for scope := range scopes.DescendingScopes(resourceScope) {
			assignments, ok := pin.GetAssignments()[scope]
			if !ok {
				continue
			}

			if !yield(scope, assignments) {
				return
			}
		}
	}
}
