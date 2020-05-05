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
import { TablePaged, Column, Cell, TextCell } from 'design/DataTable';
import { Box } from 'design';
import IconCell from './IconCell';
import UserCell from './UserCell';
import ActionCell from './ActionCell';
import CreatedCell from './CreatedCell';
import NodeCell from './NodeCell';
import { Session } from 'teleport/services/ssh';
import { Node } from 'teleport/services/nodes';

export default function SessionList(props: Props) {
  const { sessions, pageSize = 100, ...rest } = props;

  const tableProps = {
    data: sessions,
    pageSize,
    pagerPosition: 'bottom',
  };

  return (
    <Box {...rest}>
      <TablePaged {...tableProps}>
        <Column header={<Cell />} cell={<IconCell />} />
        <Column
          header={<Cell>Description</Cell>}
          cell={<Cell>Session is in progress...</Cell>}
        />
        <Column header={<Cell>Users</Cell>} cell={<UserCell />} />
        <Column header={<Cell>Node</Cell>} cell={<NodeCell />} />
        <Column header={<Cell>Started (UTC)</Cell>} cell={<CreatedCell />} />
        <Column
          columnKey="durationText"
          header={<Cell>Duration</Cell>}
          cell={<TextCell />}
        />
        <Column
          columnKey="sid"
          header={<Cell>Session ID</Cell>}
          cell={<TextCell />}
        />
        <Column header={<Cell />} cell={<ActionCell />} />
      </TablePaged>
    </Box>
  );
}

type Props = {
  sessions: Session[];
  nodes: Record<string, Node>;
  pageSize?: number;
};
