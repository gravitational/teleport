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
	"github.com/gravitational/teleport/lib/utils/set"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// PrincipalSet holds the visible (granted + requestable) and granted-only
// principals for a single dimension of access (e.g., SSH logins, AWS role ARNs,
// db_names, db_users).
type PrincipalSet struct {
	// All contains both granted and requestable principals.
	All set.Set[string]
	// Granted contains only the principals the user can use without an
	// access request.
	Granted set.Set[string]
}

// ResourcePrincipal represents a principal with a flag indicating whether
// access to it requires an access request.
type ResourcePrincipal struct {
	// Name is the principal identifier (e.g., Azure identity URI, GCP service account email).
	Name string `json:"name"`
	// RequiresRequest is true if the principal is not directly granted and
	// must be requested via an access request.
	RequiresRequest bool `json:"requiresRequest,omitempty"`
}

// ResourcePrincipalsFromSet converts a PrincipalSet to a slice of ResourcePrincipal.
func ResourcePrincipalsFromSet(ps *PrincipalSet) []ResourcePrincipal {
	if ps == nil {
		return nil
	}
	return slices.Map(ps.All.Elements(), func(name string) ResourcePrincipal {
		return ResourcePrincipal{Name: name, RequiresRequest: !ps.Granted.Contains(name)}
	})
}
