/*
Copyright 2019-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Cell, Column } from 'design/DataTable';
import Table from 'design/DataTable/Paged';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { State as ResourceState } from 'teleport/components/useResources';
import { State as RolesState } from '../useRoles';

export default function RoleList({ items, onEdit, onDelete }: Props) {
  items = items || [];
  const tableProps = { pageSize: 20, data: items };
  return (
    <Table {...tableProps}>
      <Column header={<Cell>Name</Cell>} cell={<RoleNameCell />} />
      <Column
        header={<Cell />}
        cell={<ActionCell onEdit={onEdit} onDelete={onDelete} />}
      />
    </Table>
  );
}

const RoleNameCell = props => {
  const { rowIndex, data } = props;
  const { name } = data[rowIndex];
  return <Cell>{name}</Cell>;
};

const ActionCell = props => {
  const { rowIndex, onEdit, onDelete, data } = props;
  const { id, owner } = data[rowIndex];

  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={() => onEdit(id)}>Edit...</MenuItem>
        <MenuItem disabled={owner} onClick={() => onDelete(id)}>
          Delete...
        </MenuItem>
      </MenuButton>
    </Cell>
  );
};

type Props = {
  items: RolesState['items'];
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
};
