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
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types/events"
)

// ToEventsPin converts a scopesv1.Pin to an events.ScopePin. Used in the creation of
// audit events that encode the identity parameters of an actor as part of the event.
// This function flattens the assignment tree into the simpler flat format used for audit events.
func ToEventsPin(pin *scopesv1.Pin) *events.ScopePin {
	if pin == nil {
		return nil
	}

	// TODO(fspmarshall/scopes): reevaluate how we show the pin in events. Should we convert it to the
	// new assignment tree format even though it is less readable? Keep as old flat format? Some third option?
	// For now, we flatten the tree by grouping roles by their scope of effect (where they apply).
	ea := make(map[string]*events.ScopePinnedAssignments)
	for assignment := range EnumerateAllAssignments(pin) {
		// Group roles by their scope of effect (where they apply) for the audit record
		scope := assignment.ScopeOfEffect
		if ea[scope] == nil {
			ea[scope] = &events.ScopePinnedAssignments{}
		}
		ea[scope].Roles = append(ea[scope].Roles, assignment.RoleName)
	}

	return &events.ScopePin{
		Scope:       pin.GetScope(),
		Assignments: ea,
	}
}
