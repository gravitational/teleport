/*
Copyright 2019 Gravitational, Inc.

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
import { TablePaged, Column, Cell } from 'design/DataTable';
import { Box } from 'design';
import TypeCell from './TypeCell';
import UserCell from './UserCell';
import ActionCell from './ActionCell';
import CreatedCell from './CreatedCell';
import DescCell from './DescCell';

export default function SessionList({
  sessions,
  nodes,
  pageSize = 100,
  ...rest
}) {
  return (
    <Box {...rest}>
      <TablePaged data={sessions} pageSize={pageSize} pagerPosition="bottom">
        <Column header={<Cell>Type</Cell>} cell={<TypeCell />} />
        <Column
          nodes={nodes}
          columnKey="description"
          header={<Cell>Description</Cell>}
          cell={<DescCell />}
        />
        <Column
          columnKey="createdBy"
          header={<Cell>User</Cell>}
          cell={<UserCell />}
        />
        <Column header={<Cell>Created</Cell>} cell={<CreatedCell />} />
        <Column header={<Cell />} cell={<ActionCell />} />
      </TablePaged>
    </Box>
  );
}
