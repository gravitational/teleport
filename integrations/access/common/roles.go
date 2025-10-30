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

package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/services"
)

// GetUserRoles returns the set of roles with applied traits.
func GetUserRoles(ctx context.Context, client services.RoleGetter, roleNames []string, traits trait.Traits) ([]types.Role, error) {
	var roles []types.Role
	for _, roleName := range roleNames {
		role, err := client.GetRole(ctx, roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Apply traits if provided
		if traits != nil {
			role, err = services.ApplyTraits(role, traits)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		roles = append(roles, role)
	}
	return roles, nil
}
