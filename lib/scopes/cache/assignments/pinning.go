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

package assignments

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
)

// PopulatePinnedAssignmentsForUser populates the provided scope pin with all relevant assignments related to the
// given user. The provided pin must already have its Scope field set.
func (c *AssignmentCache) PopulatePinnedAssignmentsForUser(ctx context.Context, user string, pin *scopesv1.Pin) error {
	if user == "" {
		return trace.BadParameter("missing user in scoped assignment population request")
	}
	if pin == nil {
		return trace.BadParameter("missing scope pin in assignment population request for user %q", user)
	}

	// validate the pin scope before proceeding. in theory the caller should be auth certificate generation logic which has
	// already done strong validation, but a malformed scope pin would be a bad thing to have and catching the malformed scope
	// during later pin validation steps produces confusing error messages.
	if err := scopes.WeakValidate(pin.GetScope()); err != nil {
		return trace.Errorf("invalid scope %q in assignment population request for user %q: %w", pin.GetScope(), user, err)
	}

	if pin.GetAssignmentTree() != nil {
		return trace.BadParameter("assignment population attempted with pin that already contains an assignment tree (this is a bug)")
	}

	// Track whether we've added any assignments to detect the empty case
	assignmentCount := 0

	// track the last error encountered when writing assignments. we generally want to just skip malformed assignments, but
	// if all assignments seem malformed then we want to bubble up the error.
	var lastErr error

	// all non-orthogonal assignments for this user *may* assign roles relevant to this pin
	assignments := c.cache.AllNonOrthogonalResources(pin.Scope, c.cache.WithFilter(func(assignment *scopedaccessv1.ScopedRoleAssignment) bool {
		return assignment.GetSpec().GetUser() == user
	}))

	// iterate over all potentially relevant assignments and populate the assignment tree.
	// The assignment tree encodes both Scope of Origin (from the assignment resource's scope)
	// and Scope of Effect (from the sub-assignment's scope), which is critical for proper
	// single-role evaluation ordering.
	for scope := range assignments {
		for assignment := range scope.Items() {
			// Scope of Origin is the scope of the assignment resource itself - this represents
			// the authority/provenance of the policy.
			scopeOfOrigin := assignment.GetScope()

			for subAssignment := range scopedaccess.WeakValidatedSubAssignments(assignment) {
				// Scope of Effect is the scope at which the role's privileges apply
				scopeOfEffect := subAssignment.GetScope()

				if scopes.Compare(scopeOfEffect, pin.GetScope()) == scopes.Orthogonal {
					// a non-orthogonal assignment may still have sub-assignments that are orthogonal to the pin scope
					// (e.g. an assignment at `/foo` is non-orthogonal to a pin at `/foo/bar`, but may contain a
					// sub-assignment at `/foo/bin`).
					continue
				}

				if subAssignment.GetRole() == "" {
					// some future-proofing, we don't currently support sub-assignments without a role, but may at some
					// point in the future.
					continue
				}

				// write the role assignment to the pin's assignment tree. the write function will automatically handle
				// deduplication and maintain proper tree structure for evaluation ordering.
				if err := pinning.WriteRoleAssignment(pin, pinning.RoleAssignment{
					ScopeOfOrigin: scopeOfOrigin,
					ScopeOfEffect: scopeOfEffect,
					RoleName:      subAssignment.GetRole(),
				}); err != nil {
					slog.WarnContext(ctx, "failed to write role assignment to scope pin", "role_name", subAssignment.GetRole(), "scope_of_origin", scopeOfOrigin, "scope_of_effect", scopeOfEffect, "user", user, "error", err)
					lastErr = trace.Wrap(err)
					continue
				}

				assignmentCount++
			}
		}
	}

	if assignmentCount == 0 {
		// if the assignment count is zero due to error(s) encountered during writing, return the most recent error.
		if lastErr != nil {
			return trace.Errorf("failed to populate any scoped role assignments for user %q applicable to pinned scope %q: last error: %w", user, pin.GetScope(), lastErr)
		}
		// in theory there isn't any harm in allowing pins to be created without any assignments, but we're choosing to err
		// on the side of caution for now. this limitation may be lifted later. this condition would also be caught by standard
		// strong validation, but the resulting error message would be confusing.
		// NOTE: if lifting this restriction, the equivalent check in the strong pin validation logic must also be lifted.
		return trace.NotFound("no scoped role assignments found for user %q applicable to pinned scope %q", user, pin.GetScope())
	}

	// perform a final weak validation of the pin to ensure that it is well-formed. this should be redundant since auth performs strong
	// validation of all pins prior to encoding them on certs, but its worth being defensive due to how critical scope pins are.
	if err := pinning.WeakValidate(pin); err != nil {
		return trace.Errorf("pin for scope %q was invalid post-population (this is a bug): %w", pin.GetScope(), trace.Wrap(err))
	}

	return nil
}
