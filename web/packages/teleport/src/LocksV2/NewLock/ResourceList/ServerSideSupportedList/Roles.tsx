/**
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

import React from 'react';
import Table from 'design/DataTable';

import { RoleResource } from 'teleport/services/resources';

import { renderActionCell, ServerSideListProps } from '../common';

export function Roles(props: ServerSideListProps & { roles: RoleResource[] }) {
  const {
    roles = [],
    selectedResources,
    toggleSelectResource,
    fetchStatus,
  } = props;

  return (
    <Table
      data={roles}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          altKey: 'action-btn',
          render: ({ name }) =>
            renderActionCell(Boolean(selectedResources.role[name]), () =>
              toggleSelectResource({ kind: 'role', targetValue: name })
            ),
        },
      ]}
      emptyText="No Roles Found"
      disableFilter
      fetching={{
        fetchStatus,
      }}
    />
  );
}
