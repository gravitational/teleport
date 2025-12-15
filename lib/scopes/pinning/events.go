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
func ToEventsPin(pin *scopesv1.Pin) *events.ScopePin {
	if pin == nil {
		return nil
	}

	var ea map[string]*events.ScopePinnedAssignments
	if assignments := pin.GetAssignments(); assignments != nil {
		ea = make(map[string]*events.ScopePinnedAssignments, len(assignments))
		for scope, assignment := range assignments {
			ea[scope] = &events.ScopePinnedAssignments{
				Roles: assignment.GetRoles(),
			}
		}
	}

	return &events.ScopePin{
		Scope:       pin.GetScope(),
		Assignments: ea,
	}
}
