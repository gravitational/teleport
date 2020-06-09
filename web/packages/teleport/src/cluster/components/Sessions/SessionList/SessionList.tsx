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
import { Session } from 'teleport/services/ssh';
import CardEmpty from 'teleport/components/CardEmpty';
import DescCell from './DescCell';
import UserCell from './UserCell';
import ActionCell from './ActionCell';
import NodeCell from './NodeCell';

export default function SessionList(props: Props) {
  const { sessions, pageSize = 100, ...rest } = props;
  const tableProps = {
    data: sessions,
    pageSize,
  };

  if (sessions.length === 0)
    return <CardEmpty title="No Active Sessions Found" />;

  return (
    <Box {...rest}>
      <TablePaged {...tableProps}>
        <Column header={<Cell>Description</Cell>} cell={<DescCell />} />
        <Column
          columnKey="sid"
          header={<Cell>Session ID</Cell>}
          cell={<TextCell />}
        />
        <Column header={<Cell>Users</Cell>} cell={<UserCell />} />
        <Column header={<Cell>Node</Cell>} cell={<NodeCell />} />
        <Column
          columnKey="durationText"
          header={<Cell>Duration</Cell>}
          cell={<TextCell />}
        />
        <Column header={<Cell />} cell={<ActionCell />} />
      </TablePaged>
    </Box>
  );
}

type Props = {
  sessions: Session[];
  pageSize?: number;
};
