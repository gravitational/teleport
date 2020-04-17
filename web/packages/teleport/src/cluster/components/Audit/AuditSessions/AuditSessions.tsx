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
import styled from 'styled-components';
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import { NavLink } from 'react-router-dom';
import { ButtonPrimary, Flex } from 'design';
import { displayDateTime } from 'shared/services/loc';
import * as Icons from 'design/Icon';
import * as DataTable from 'design/DataTable';
import {
  usePages,
  Pager,
  StyledPanel,
  StyledButtons,
} from 'design/DataTable/Paged';
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
        created: DataTable.SortTypes.ASC,
      },
    };
  });

  // sort and filter
  const data = React.useMemo(() => {
    const { colSortDirs, search } = state;
    const rows = events.filter(e => e.code === 'T2004I').map(makeRows);

    const filtered = rows.filter(obj =>
      isMatch(obj, search, {
        searchableProps,
        cb: null,
      })
    );

    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    const sorted = sortBy(filtered, columnKey);
    if (sortDir === DataTable.SortTypes.ASC) {
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

  const pagedState = usePages({ pageSize, data });

  return (
    <>
      <CustomStyledPanel
        alignItems="center"
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
        justifyContent="space-between"
      >
        <InputSearch height="30px" mr="2" onChange={onSearchChange} />
        <Flex alignItems="center" justifyContent="flex-end">
          {pagedState.hasPages && <Pager {...pagedState} />}
        </Flex>
      </CustomStyledPanel>
      <DataTable.Table data={pagedState.data}>
        <DataTable.Column
          header={<DataTable.Cell>Session ID</DataTable.Cell>}
          cell={<SidCell />}
        />
        <DataTable.Column
          columnKey="users"
          header={<DataTable.Cell>User(s)</DataTable.Cell>}
          cell={<DataTable.TextCell />}
        />
        <DataTable.Column
          columnKey="hostname"
          header={<DataTable.Cell>Node</DataTable.Cell>}
          cell={<DataTable.TextCell />}
        />
        <DataTable.Column
          columnKey="created"
          header={
            <DataTable.SortHeaderCell
              sortDir={state.colSortDirs.created}
              onSortChange={onSortChange}
              title="Created"
            />
          }
          cell={<CreatedCell />}
        />
        <DataTable.Column
          columnKey="durationText"
          header={<DataTable.Cell>Duration</DataTable.Cell>}
          cell={<DataTable.TextCell />}
        />
        <DataTable.Column header={<DataTable.Cell />} cell={<PlayCell />} />
      </DataTable.Table>
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
  return <DataTable.Cell>{row.createdText}</DataTable.Cell>;
}

function SidCell(props) {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  return (
    <DataTable.Cell>
      <div style={{ display: 'flex', alignItems: 'center' }}>
        <Icons.Cli
          p="1"
          mr="3"
          bg="bgTerminal"
          fontSize="4"
          style={{
            borderRadius: '50%',
            border: 'solid 2px green',
          }}
        />
        {row.sid}
      </div>
    </DataTable.Cell>
  );
}

const PlayCell = props => {
  const { rowIndex, data } = props;
  const row = data[rowIndex] as Row;
  const url = cfg.getSessionAuditPlayerRoute(row);
  return (
    <DataTable.Cell align="right">
      <ButtonPrimary as={NavLink} to={url} size="small">
        Play
      </ButtonPrimary>
    </DataTable.Cell>
  );
};

const CustomStyledPanel = styled(StyledPanel)`
  ${StyledButtons} {
    margin-left: ${props => `${props.theme.space[3]}px`};
  }
`;
