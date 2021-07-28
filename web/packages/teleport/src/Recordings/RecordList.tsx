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
import moment from 'moment';
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import { ButtonBorder } from 'design';
import { displayDateTime } from 'shared/services/loc';
import * as Table from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import { SessionEnd } from 'teleport/services/audit/types';
import cfg from 'teleport/config';
import { State } from 'teleport/useAuditEvents';

type SortCols = 'created' | 'duration';
type SortState = {
  [key in SortCols]?: string;
};

export default function RecordList(props: Props) {
  const {
    clusterId,
    searchValue,
    pageSize,
    events,
    fetchMore,
    fetchStatus,
  } = props;
  const [colSortDirs, setSort] = React.useState<SortState>(() => {
    return {
      created: Table.SortTypes.ASC,
    };
  });

  // sort and filter
  const data = React.useMemo(() => {
    const rows = events
      .filter(e => e.code === 'T2004I')
      .map(makeRows(clusterId));

    const filtered = rows.filter(obj =>
      isMatch(obj, searchValue, {
        searchableProps,
        cb: null,
      })
    );

    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    const sorted = sortBy(filtered, columnKey);
    if (sortDir === Table.SortTypes.ASC) {
      return sorted.reverse();
    }

    return sorted;
  }, [colSortDirs, events, searchValue]);

  function onSortChange(columnKey: SortCols, sortDir: string) {
    setSort({ [columnKey]: sortDir });
  }

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
            sortDir={colSortDirs.duration}
            onSortChange={onSortChange}
            title="Duration"
          />
        }
        cell={<DurationCell />}
      />
      <Table.Column
        columnKey="created"
        header={
          <Table.SortHeaderCell
            sortDir={colSortDirs.created}
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
      <Table.Column header={<Table.Cell />} cell={<PlayCell />} />
    </PagedTable>
  );
}

const makeRows = (clusterId: string) => (event: SessionEnd) => {
  const { time, raw } = event;
  const users = raw?.participants || [];
  const rawEvent = event.raw;

  let durationText = '';
  let duration = 0;
  if (rawEvent.session_start && rawEvent.session_stop) {
    duration = moment(rawEvent.session_stop).diff(rawEvent.session_start);
    durationText = moment.duration(duration).humanize();
  }

  let hostname = raw.server_hostname || 'N/A';
  // For Kubernetes sessions, put the full pod name as 'hostname'.
  if (raw.proto === 'kube') {
    hostname = `${raw.kubernetes_cluster}/${raw.kubernetes_pod_namespace}/${raw.kubernetes_pod_name}`;
  }

  // Description set to play for interactive so users can search by "play".
  let description = raw.interactive ? 'play' : 'non-interactive';
  if (raw.session_recording === 'off') {
    description = 'recording disabled';
  }

  return {
    clusterId,
    duration,
    durationText,
    sid: raw.sid,
    created: time,
    createdText: displayDateTime(time),
    users: users.join(', '),
    hostname: hostname,
    description,
  };
};

type Row = ReturnType<ReturnType<typeof makeRows>>;

function CreatedCell(props) {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  return <Table.Cell>{row.createdText}</Table.Cell>;
}

function DurationCell(props) {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  return <Table.Cell>{row.durationText}</Table.Cell>;
}

function SidCell(props) {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  return <Table.Cell>{row.sid}</Table.Cell>;
}

const PlayCell = props => {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;

  if (row.description !== 'play') {
    return (
      <Table.Cell align="right" style={{ color: '#9F9F9F' }}>
        {row.description}
      </Table.Cell>
    );
  }

  const url = cfg.getSessionAuditPlayerRoute(row);
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
  events: State['events'];
  clusterId: State['clusterId'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
};

const searchableProps = [
  'sid',
  'createdText',
  'users',
  'durationText',
  'hostname',
  'description',
];
