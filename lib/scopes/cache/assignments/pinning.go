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

	"github.com/gravitational/trace"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache"
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

	if pin.GetAssignments() == nil {
		pin.Assignments = make(map[string]*scopesv1.PinnedAssignments)
	}

	if len(pin.GetAssignments()) != 0 {
		return trace.BadParameter("assignment population attempted with pin that already contains assignments (this is a bug)")
	}

	// rather than just normalizing/deduplicating the individual subassignments manually, its
	// easier to just build an ephemeral scope cache and let it do the work for us.
	type entry struct {
		scope string
		role  string
		key   string
	}

	ec := cache.Must(cache.Config[entry, string]{
		Scope: func(e entry) string {
			return e.scope
		},
		Key: func(e entry) string {
			return e.key
		},
	})

	// all non-orthogonal assignments for this user *may* assign roles relevant to this pin
	assignments := c.cache.AllNonOrthogonalResources(pin.Scope, c.cache.WithFilter(func(assignment *scopedaccessv1.ScopedRoleAssignment) bool {
		return assignment.GetSpec().GetUser() == user
	}))

	// iterate over all potentially relevant assignments and store each assigned role in the ephemeral cache
	for scope := range assignments {
		for assignment := range scope.Items() {
			for subAssignment := range scopedaccess.WeakValidatedSubAssignments(assignment) {
				if scopes.Compare(subAssignment.GetScope(), pin.GetScope()) == scopes.Orthogonal {
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
				ec.Put(entry{
					scope: subAssignment.GetScope(),
					role:  subAssignment.GetRole(),
					key:   subAssignment.GetRole() + "@" + subAssignment.GetScope(), // to satisfy cmp.Ordered
				})
			}
		}
	}

	// iterate the full contents of the ephemeral cache and populate the pin with the relevant assignments.
	for scope := range ec.ResourcesSubjectToPolicyScope(scopes.Root) {
		var roles []string
		for entry := range scope.Items() {
			roles = append(roles, entry.role)
		}
		pin.Assignments[scope.Scope()] = &scopesv1.PinnedAssignments{
			Roles: roles,
		}
	}

	if len(pin.GetAssignments()) == 0 {
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

// PopulatePinnedAssignmentsForBot populates the provided scope pin with all relevant assignments related to the
// given bot. The provided pin must already have its Scope field set.
//
// botScope should be the scope at which the bot in question is defined. This
// must have already been validated to ensure this matches join token.
//
// TODO: Noah - Speak to Forrest and determine if this should be merged w/
// the function above? or if separate is clearer.
//
// This deviates from the user implementation in the following way:
// 1) Filters out assignments for users - i.e that don't specify bot_name and bot_scope.
// 2) Ensures that the bot_scope within the SRA matches the specified bot_scope, else ignores.
// 3) Ensures that sub-assignments are to a scope that is equiv or descendent to the Bot scope.
// 4) Ensures that the pin scope is also equiv or descendent to the bot scope.
// 3&4 will be relaxed with the introduction of cross-scope privilege for MWI.
func (c *AssignmentCache) PopulatePinnedAssignmentsForBot(
	ctx context.Context, botName string, botScope string, pin *scopesv1.Pin,
) error {
	if botName == "" {
		return trace.BadParameter("missing bot name in scoped assignment population request")
	}
	if botScope == "" {
		return trace.BadParameter("missing bot scope in scoped assignment population request for bot %q", botName)
	}
	if pin == nil {
		return trace.BadParameter("missing scope pin in assignment population request for user %q", botName)
	}

	// validate the pin scope before proceeding. in theory the caller should be auth certificate generation logic which has
	// already done strong validation, but a malformed scope pin would be a bad thing to have and catching the malformed scope
	// during later pin validation steps produces confusing error messages.
	if err := scopes.WeakValidate(pin.GetScope()); err != nil {
		return trace.Errorf("invalid scope %q in assignment population request for user %q: %w", pin.GetScope(), botName, err)
	}
	if err := scopes.WeakValidate(botScope); err != nil {
		return trace.Errorf("invalid bot scope %q in assignment population request for bot %q: %w", botScope, botName, err)
	}

	// Ensure the scope we are pinning to it descendent or equiv to the bot scope.
	// nb: This restriction may be relaxed w/ introduction of cross-scope privilege.
	if !scopes.PolicyAssignmentScope(pin.Scope).IsSubjectToPolicyResourceScope(botScope) {
		return trace.BadParameter(
			"pinned scope %q is not subject to bot scope %q in assignment population request for bot %q",
			pin.GetScope(), botScope, botName,
		)
	}

	if pin.GetAssignments() == nil {
		pin.Assignments = make(map[string]*scopesv1.PinnedAssignments)
	}

	if len(pin.GetAssignments()) != 0 {
		return trace.BadParameter("assignment population attempted with pin that already contains assignments (this is a bug)")
	}

	// rather than just normalizing/deduplicating the individual subassignments manually, its
	// easier to just build an ephemeral scope cache and let it do the work for us.
	type entry struct {
		scope string
		role  string
		key   string
	}

	ec := cache.Must(cache.Config[entry, string]{
		Scope: func(e entry) string {
			return e.scope
		},
		Key: func(e entry) string {
			return e.key
		},
	})

	// all non-orthogonal assignments for this bot *may* assign roles relevant to this pin
	assignments := c.cache.AllNonOrthogonalResources(pin.Scope, c.cache.WithFilter(func(assignment *scopedaccessv1.ScopedRoleAssignment) bool {
		matchesBotName := assignment.GetSpec().GetBotName() == botName
		// ignore assignments where actual bot scope mismatches bot scope
		// specified in SRA. this mitigates name reuse attacks across scopes.
		matchesBotScope := assignment.GetSpec().GetBotScope() == botScope
		return matchesBotName && matchesBotScope
	}))

	// iterate over all potentially relevant assignments and store each assigned role in the ephemeral cache
	for scope := range assignments {
		for assignment := range scope.Items() {
			for subAssignment := range scopedaccess.WeakValidatedSubAssignments(assignment) {
				if scopes.Compare(subAssignment.GetScope(), pin.GetScope()) == scopes.Orthogonal {
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
				// ignore assignments where assigned scope is not descendent or equiv to bot scope.
				if !scopes.PolicyAssignmentScope(subAssignment.GetScope()).IsSubjectToPolicyResourceScope(botScope) {
					continue
				}

				ec.Put(entry{
					scope: subAssignment.GetScope(),
					role:  subAssignment.GetRole(),
					key:   subAssignment.GetRole() + "@" + subAssignment.GetScope(), // to satisfy cmp.Ordered
				})
			}
		}
	}

	// iterate the full contents of the ephemeral cache and populate the pin with the relevant assignments.
	for scope := range ec.ResourcesSubjectToPolicyScope(scopes.Root) {
		var roles []string
		for entry := range scope.Items() {
			roles = append(roles, entry.role)
		}
		pin.Assignments[scope.Scope()] = &scopesv1.PinnedAssignments{
			Roles: roles,
		}
	}

	if len(pin.GetAssignments()) == 0 {
		// in theory there isn't any harm in allowing pins to be created without any assignments, but we're choosing to err
		// on the side of caution for now. this limitation may be lifted later. this condition would also be caught by standard
		// strong validation, but the resulting error message would be confusing.
		// NOTE: if lifting this restriction, the equivalent check in the strong pin validation logic must also be lifted.
		return trace.NotFound("no scoped role assignments found for bot %q applicable to pinned scope %q", botName, pin.GetScope())
	}

	// perform a final weak validation of the pin to ensure that it is well-formed. this should be redundant since auth performs strong
	// validation of all pins prior to encoding them on certs, but its worth being defensive due to how critical scope pins are.
	if err := pinning.WeakValidate(pin); err != nil {
		return trace.Errorf("pin for scope %q was invalid post-population (this is a bug): %w", pin.GetScope(), trace.Wrap(err))
	}

	return nil
}
