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

import React, { useState } from 'react';
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import { ButtonBorder } from 'design';
import { displayDateTime } from 'shared/services/loc';
import * as Table from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import cfg from 'teleport/config';
import { State } from './useRecordings';
import { Recording } from 'teleport/services/recordings';

export default function RecordingsList(props: Props) {
  const {
    recordings,
    clusterId,
    searchValue,
    pageSize,
    fetchMore,
    fetchStatus,
  } = props;
  const [sortDir, setSortDir] = useState<Record<string, string>>(() => {
    return {
      createdDate: Table.SortTypes.ASC,
    };
  });

  function sortAndFilter(search: string) {
    const filtered = recordings.filter(obj =>
      isMatch(obj, search, {
        searchableProps,
        cb: null,
      })
    );

    const columnKey = Object.getOwnPropertyNames(sortDir)[0];
    const sorted = sortBy(filtered, columnKey);
    if (sortDir[columnKey] === Table.SortTypes.ASC) {
      return sorted.reverse();
    }

    return sorted;
  }

  function onSortChange(columnKey: string, sortDir: string) {
    setSortDir({ [columnKey]: sortDir });
  }

  const data = sortAndFilter(searchValue);

  const tableProps = { pageSize, data, fetchMore, fetchStatus };

  return (
    <PagedTable {...tableProps}>
      <Table.Column
        columnKey="hostname"
        header={<Table.Cell>Node</Table.Cell>}
        cell={<Table.TextCell />}
      />
      <Table.Column
        columnKey="users"
        header={<Table.Cell>User(s)</Table.Cell>}
        cell={<Table.TextCell style={{ wordBreak: 'break-word' }} />}
      />
      <Table.Column
        columnKey="duration"
        header={
          <Table.SortHeaderCell
            sortDir={sortDir.duration}
            onSortChange={onSortChange}
            title="Duration"
          />
        }
        cell={<DurationCell />}
      />
      <Table.Column
        columnKey="createdDate"
        header={
          <Table.SortHeaderCell
            sortDir={sortDir.createdDate}
            onSortChange={onSortChange}
            title="Created"
          />
        }
        cell={<CreatedCell />}
      />
      <Table.Column
        header={<Table.Cell>Session ID</Table.Cell>}
        cell={<SidCell />}
      />
      <Table.Column
        header={<Table.Cell />}
        cell={<PlayCell clusterId={clusterId} />}
      />
    </PagedTable>
  );
}

function CreatedCell(props) {
  const { rowIndex, data } = props;
  const { createdDate } = data[rowIndex] as Recording;
  return <Table.Cell>{displayDateTime(createdDate)}</Table.Cell>;
}

function DurationCell(props) {
  const { rowIndex, data } = props;
  const { durationText } = data[rowIndex] as Recording;
  return <Table.Cell>{durationText}</Table.Cell>;
}

function SidCell(props) {
  const { rowIndex, data } = props;
  const { sid } = data[rowIndex] as Recording;
  return <Table.Cell>{sid}</Table.Cell>;
}

const PlayCell = props => {
  const { rowIndex, data, clusterId } = props;
  const { description, sid } = data[rowIndex] as Recording;

  if (description !== 'play') {
    return (
      <Table.Cell align="right" style={{ color: '#9F9F9F' }}>
        {description}
      </Table.Cell>
    );
  }

  const url = cfg.getSessionAuditPlayerRoute({ clusterId, sid });
  return (
    <Table.Cell align="right">
      <ButtonBorder
        kind="primary"
        as="a"
        href={url}
        width="80px"
        target="_blank"
        size="small"
      >
        Play
      </ButtonBorder>
    </Table.Cell>
  );
};

type Props = {
  pageSize?: number;
  searchValue: State['searchValue'];
  recordings: State['recordings'];
  clusterId: State['clusterId'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
};

const searchableProps = [
  'sid',
  'createdDate',
  'users',
  'durationText',
  'hostname',
  'description',
];
