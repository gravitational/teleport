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
