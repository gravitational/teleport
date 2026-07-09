/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package access

import (
	"slices"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/scopes"
)

// MatcHSecondaryAssignmentFilters provides a single source of truth for a subset of the matching logic needed to implement
// the ListScopedRoleAssignments RPC. How the primary scope filter functions differs between cache/backend impls, but the
// secondary filters (e.g. user name) function identically, so we centralize them here.
func MatchSecondaryAssignmentFilters(req *scopedaccessv1.ListScopedRoleAssignmentsRequest, assignment *scopedaccessv1.ScopedRoleAssignment) bool {
	if req.GetUser() != "" && assignment.GetSpec().GetUser() != req.GetUser() {
		return false
	}

	if req.GetRole() != "" {
		if !slices.ContainsFunc(assignment.GetSpec().GetAssignments(), func(a *scopedaccessv1.Assignment) bool {
			return a.GetRole() == req.GetRole()
		}) {
			return false
		}
	}

	if asf := req.GetAssignedScopeFilter(); !scopes.IsMatchAll(asf) {
		// we only evaluate non-wildcard filters here because we don't want to suppress assignments with
		// empty sub-assignment sets unless the filter is restrictive.
		if !slices.ContainsFunc(assignment.GetSpec().GetAssignments(), func(a *scopedaccessv1.Assignment) bool {
			return scopes.MatchScope(asf, a.GetScope())
		}) {
			return false
		}
	}

	return true
}
