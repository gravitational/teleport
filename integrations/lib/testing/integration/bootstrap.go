/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package integration

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type Bootstrap struct {
	resources []types.Resource
}

func (bootstrap *Bootstrap) Add(resource types.Resource) {
	bootstrap.resources = append(bootstrap.resources, resource)
}

func (bootstrap *Bootstrap) Resources() []types.Resource {
	return bootstrap.resources
}

func (bootstrap *Bootstrap) AddUserWithRoles(name string, roles ...string) (types.User, error) {
	user, err := types.NewUser(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles(roles)
	bootstrap.Add(user)
	return user, nil
}

func (bootstrap *Bootstrap) AddRole(name string, spec types.RoleSpecV6) (types.Role, error) {
	role, err := types.NewRole(name, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bootstrap.Add(role)
	return role, nil
}
