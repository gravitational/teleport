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
import isMatch from 'design/utils/match';
import { ButtonBorder, Flex } from 'design';
import { displayDateTime } from 'shared/services/loc';
import * as Icons from 'design/Icon';
import * as Table from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import InputSearch from 'teleport/components/InputSearch';
import { Event, SessionEnd } from 'teleport/services/audit/types';
import cfg from 'teleport/config';

type SessionListProps = {
  events: Event[];
  pageSize?: number;
};

const searchableProps = [
  'sid',
  'createdText',
  'users',
  'durationText',
  'hostname',
];

export default function SessionList(props: SessionListProps) {
  const { pageSize, events } = props;
  const [state, setState] = React.useState(() => {
    return {
      search: '',
      colSortDirs: {
        created: Table.SortTypes.ASC,
      },
    };
  });

  // sort and filter
  const data = React.useMemo(() => {
    const { colSortDirs, search } = state;
    const rows = events
      .filter(e => e.code === 'T2004I' && e.raw.interactive)
      .map(makeRows);

    const filtered = rows.filter(obj =>
      isMatch(obj, search, {
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
  }, [state, events]);

  function onSortChange(columnKey: string, sortDir: string) {
    setState({
      ...state,
      colSortDirs: { [columnKey]: sortDir } as any,
    });
  }

  function onSearchChange(value: string) {
    setState({
      ...state,
      search: value,
    });
  }

  const tableProps = {
    pageSize,
    data,
  };

  return (
    <>
      <Flex mb={4} alignItems="center" justifyContent="flex-start">
        <InputSearch height="30px" mr="2" onChange={onSearchChange} />
      </Flex>
      <PagedTable {...tableProps}>
        <Table.Column
          header={<Table.Cell>Session ID</Table.Cell>}
          cell={<SidCell />}
        />
        <Table.Column
          columnKey="users"
          header={<Table.Cell>User(s)</Table.Cell>}
          cell={<Table.TextCell style={{ wordBreak: 'break-word' }} />}
        />
        <Table.Column
          columnKey="hostname"
          header={<Table.Cell>Node</Table.Cell>}
          cell={<Table.TextCell />}
        />
        <Table.Column
          columnKey="created"
          header={
            <Table.SortHeaderCell
              sortDir={state.colSortDirs.created}
              onSortChange={onSortChange}
              title="Created"
            />
          }
          cell={<CreatedCell />}
        />
        <Table.Column
          columnKey="durationText"
          header={<Table.Cell>Duration</Table.Cell>}
          cell={<Table.TextCell />}
        />
        <Table.Column header={<Table.Cell />} cell={<PlayCell />} />
      </PagedTable>
    </>
  );
}

function makeRows(event: SessionEnd) {
  const { time, raw } = event;
  const users = raw?.participants || [];
  return {
    sid: raw.sid,
    created: time,
    createdText: displayDateTime(time),
    users: users.join(', '),
    durationText: 'not implemented',
    hostname: raw.server_hostname,
  };
}

type Row = ReturnType<typeof makeRows>;

function CreatedCell(props) {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  return <Table.Cell>{row.createdText}</Table.Cell>;
}

function SidCell(props) {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  return (
    <Table.Cell>
      <div style={{ display: 'flex', alignItems: 'center' }}>
        <Icons.Cli
          p="1"
          mr="3"
          bg="bgTerminal"
          fontSize="2"
          style={{
            borderRadius: '50%',
            border: 'solid 2px #512FC9',
          }}
        />
        {row.sid}
      </div>
    </Table.Cell>
  );
}

const PlayCell = props => {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  const url = cfg.getSessionAuditPlayerRoute(row);
  return (
    <Table.Cell align="right">
      <ButtonBorder as="a" href={url} target="_blank" size="small">
        Play
      </ButtonBorder>
    </Table.Cell>
  );
};
