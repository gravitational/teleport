/*
Copyright 2019-2020 Gravitational, Inc.

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
import { RoleNameCell, ActionCell } from './RoleListCells';

export default function RoleList({ items, onEdit, onDelete }) {
  items = items || [];
  return (
    <Table pageSize={20} data={items}>
      <Column header={<Cell>Name</Cell>} cell={<RoleNameCell />} />
      <Column
        header={<Cell />}
        cell={<ActionCell onEdit={onEdit} onDelete={onDelete} />}
      />
    </Table>
  );
}
