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
import { sortBy } from 'lodash';
import {
  TablePaged,
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
} from 'design/DataTable';
import { ButtonSecondary, Text } from 'design';
import Status from './../ClusterStatus';

function TableView({ clusters, pageSize = 500 }) {
  const [sortDir, setSortDir] = React.useState({
    clusterId: SortTypes.ASC,
  });

  function sort(clusters) {
    const columnKey = Object.getOwnPropertyNames(sortDir)[0];
    const sorted = sortBy(clusters, columnKey);
    if (sortDir[columnKey] === SortTypes.DESC) {
      return sorted.reverse();
    }

    return sorted;
  }

  function onSortChange(columnKey, sortDir) {
    setSortDir({ [columnKey]: sortDir });
  }

  const data = sort(clusters);

  return (
    <TablePaged data={data} pageSize={pageSize}>
      <Column
        columnKey="status"
        header={
          <SortHeaderCell
            sortDir={sortDir.status}
            onSortChange={onSortChange}
            title="Status"
          />
        }
        cell={<StatusCell />}
      />
      <Column
        columnKey="clusterId"
        header={
          <SortHeaderCell
            sortDir={sortDir.clusterId}
            onSortChange={onSortChange}
            title="Name"
          />
        }
        cell={<NameCell />}
      />
      <Column
        columnKey="version"
        header={
          <SortHeaderCell
            sortDir={sortDir.version}
            onSortChange={onSortChange}
            title="Version"
          />
        }
        cell={<TextCell />}
      />
      <Column
        columnKey="nodes"
        header={
          <SortHeaderCell
            sortDir={sortDir.nodes}
            onSortChange={onSortChange}
            title="Nodes"
          />
        }
        cell={<TextCell />}
      />
      <Column
        columnKey="connected"
        header={
          <SortHeaderCell
            sortDir={sortDir.connected}
            onSortChange={onSortChange}
            title="Connected"
          />
        }
        cell={<ConnectedCell />}
      />
      <Column header={<Cell />} cell={<ActionCell />} />
    </TablePaged>
  );
}

export function NameCell({ rowIndex, data }) {
  const { clusterId } = data[rowIndex];
  return (
    <Cell>
      <Text typography="h5">{clusterId}</Text>
    </Cell>
  );
}

function ConnectedCell({ rowIndex, data }) {
  const { connectedText } = data[rowIndex];
  return <Cell>{connectedText}</Cell>;
}

function StatusCell({ rowIndex, data }) {
  const { status } = data[rowIndex];
  return (
    <Cell>
      <Status ml="3" status={status} />
    </Cell>
  );
}

function ActionCell({ rowIndex, data }) {
  const { url } = data[rowIndex];
  return (
    <Cell align="right">
      <ButtonSecondary
        as="a"
        target="_blank"
        href={url}
        size="small"
        width="90px"
        children="view"
      />
    </Cell>
  );
}

export default TableView;
