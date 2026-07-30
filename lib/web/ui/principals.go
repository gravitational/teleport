/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package ui

import (
	"slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/set"
)

// PrincipalSet holds the visible (granted + requestable) and granted-only
// principals for a single dimension of access (e.g., SSH logins, AWS role ARNs).
type PrincipalSet struct {
	// All contains both granted and requestable principals.
	All set.Set[string]
	// Granted contains only the principals the user can use without an
	// access request.
	Granted set.Set[string]
}

// ResourcePrincipalSet is one principal dimension of a resource.
type ResourcePrincipalSet struct {
	// PrincipalType is the dimension's inline constraint key
	// (e.g. "logins", "db_users", "kube_groups").
	PrincipalType string `json:"principalType"`
	// Granted holds values usable without an access request, sorted.
	Granted []string `json:"granted,omitempty"`
	// Requestable holds values usable only via access request, sorted.
	Requestable []string `json:"requestable,omitempty"`
	// ByRole optionally attributes values to the roles granting them, for
	// kinds whose dimensions must be co-granted by a single role.
	ByRole []RolePrincipalValues `json:"byRole,omitempty"`
}

// RolePrincipalValues is the subset of one principal dimension's values
// attributed to their granting role.
type RolePrincipalValues struct {
	// Role is the granting role's name.
	Role string `json:"role"`
	// RequiresRequest is set when the role is not held and must be requested.
	RequiresRequest bool `json:"requiresRequest,omitempty"`
	// Values are the principals the role grants for the parent dimension.
	Values []string `json:"values,omitempty"`
}

// MakeResourcePrincipalSet builds one principal dimension from a computed
// granted/visible split. Returns zero-valued set when ps is nil.
func MakeResourcePrincipalSet(principalType string, ps *PrincipalSet) ResourcePrincipalSet {
	out := ResourcePrincipalSet{PrincipalType: principalType}
	if ps == nil {
		return out
	}
	for _, v := range ps.All.Elements() {
		if ps.Granted.Contains(v) {
			out.Granted = append(out.Granted, v)
		} else {
			out.Requestable = append(out.Requestable, v)
		}
	}
	slices.Sort(out.Granted)
	slices.Sort(out.Requestable)
	return out
}

// MakeResourcePrincipalSets converts principal sets from an enriched
// resource into their web form, one entry per dimension. An empty
// input yields nil.
func MakeResourcePrincipalSets(sets []types.ResourcePrincipalSet) []ResourcePrincipalSet {
	if len(sets) == 0 {
		return nil
	}
	out := make([]ResourcePrincipalSet, 0, len(sets))
	for _, ps := range sets {
		byRole := make([]RolePrincipalValues, 0, len(ps.ByRole))
		for _, br := range ps.ByRole {
			byRole = append(byRole, RolePrincipalValues{
				Role:            br.Role,
				RequiresRequest: br.RequiresRequest,
				Values:          br.Values,
			})
		}
		if len(byRole) == 0 {
			byRole = nil
		}
		out = append(out, ResourcePrincipalSet{
			PrincipalType: ps.PrincipalType,
			Granted:       ps.Granted,
			Requestable:   ps.Requestable,
			ByRole:        byRole,
		})
	}
	return out
}
