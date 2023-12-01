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
import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { State as ResourceState } from 'teleport/components/useResources';

import { State as RolesState } from '../useRoles';

export default function RoleList({
  items = [],
  pageSize = 20,
  onEdit,
  onDelete,
}: Props) {
  return (
    <Table
      data={items}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          altKey: 'options-btn',
          render: ({ id }) => (
            <ActionCell id={id} onEdit={onEdit} onDelete={onDelete} />
          ),
        },
      ]}
      emptyText="No Roles Found"
      pagination={{ pageSize }}
      isSearchable
    />
  );
}

const ActionCell = ({
  id,
  onEdit,
  onDelete,
}: {
  id: string;
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
}) => {
  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={() => onEdit(id)}>Edit...</MenuItem>
        <MenuItem onClick={() => onDelete(id)}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};

type Props = {
  items: RolesState['items'];
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
  pageSize?: number;
};
