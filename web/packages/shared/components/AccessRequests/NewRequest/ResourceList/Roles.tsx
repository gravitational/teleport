/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React from 'react';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Roles(props: ListProps & { roles: string[] }) {
  const { roles = [], addedResources, addOrRemoveResource } = props;

  return (
    <StyledTable
      data={roles.map(role => ({ role }))}
      pagination={{ pagerPosition: 'top', pageSize: 10 }}
      isSearchable={true}
      columns={[
        {
          key: 'role',
          headerText: 'Role Name',
          isSortable: true,
        },
        {
          altKey: 'action-btn',
          render: ({ role }) =>
            renderActionCell(Boolean(addedResources.role[role]), () =>
              addOrRemoveResource('role', role)
            ),
        },
      ]}
      emptyText="No Requestable Roles Found"
    />
  );
}
