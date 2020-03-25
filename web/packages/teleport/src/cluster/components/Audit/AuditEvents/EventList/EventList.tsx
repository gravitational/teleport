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
import * as DataTable from 'design/DataTable';
import { Flex } from 'design';
import { ActionCell, TimeCell, DescCell } from './EventListCells';
import TypeCell from './EventTypeCell';
import { Event } from 'teleport/services/audit';
import EventDialog from '../../EventDialog';
import InputSearch from 'teleport/components/InputSearch';
import {
  usePages,
  Pager,
  StyledPanel,
  StyledButtons,
} from 'design/DataTable/Paged';

export default function EventList(props: EventListProps) {
  const { events = [], search = '', onSearchChange } = props;
  const [state, setState] = React.useState<EventListState>(() => {
    return {
      searchableProps: ['codeDesc', 'message', 'user', 'time'],
      detailsToShow: null,
      colSortDirs: {
        time: DataTable.SortTypes.ASC,
      },
    };
  });

  function onSortChange(columnKey: string, sortDir: string) {
    setState({
      ...state,
      colSortDirs: { [columnKey]: sortDir },
    });
  }

  function showDetails(detailsToShow: Event) {
    setState({
      ...state,
      detailsToShow,
    });
  }

  function closeDetails() {
    setState({
      ...state,
      detailsToShow: null,
    });
  }

  // sort and filter
  const data = React.useMemo(() => {
    const { colSortDirs, searchableProps } = state;
    const filtered = events.filter(obj =>
      isMatch(obj, search, {
        searchableProps: searchableProps,
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
  }, [state, events, search]);

  // paginate
  const pagging = usePages({ pageSize: 20, data });

  const { detailsToShow, colSortDirs } = state;
  return (
    <React.Fragment>
      <CustomStyledPanel
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
        justifyContent="space-between"
      >
        <InputSearch height="30px" mr="3" onChange={onSearchChange} />
        {pagging.hasPages && (
          <Flex alignItems="center" justifyContent="flex-end">
            <Pager {...pagging} />
          </Flex>
        )}
      </CustomStyledPanel>
      <DataTable.Table data={pagging.data}>
        <DataTable.Column
          columnKey="codeDesc"
          cell={<TypeCell />}
          header={
            <DataTable.SortHeaderCell
              sortDir={colSortDirs.codeDesc}
              onSortChange={onSortChange}
              title="Type"
            />
          }
        />
        <DataTable.Column
          columnKey="message"
          header={<DataTable.Cell>Description</DataTable.Cell>}
          cell={<DescCell style={{ wordBreak: 'break-all' }} />}
        />
        <DataTable.Column
          columnKey="user"
          header={
            <DataTable.SortHeaderCell
              sortDir={colSortDirs.user}
              onSortChange={onSortChange}
              title="User"
            />
          }
          cell={<DataTable.TextCell style={{ minWidth: '48px' }} />}
        />
        <DataTable.Column
          columnKey="time"
          header={
            <DataTable.SortHeaderCell
              sortDir={colSortDirs.time}
              onSortChange={onSortChange}
              title="Created"
            />
          }
          cell={<TimeCell />}
        />
        <DataTable.Column
          header={<DataTable.Cell />}
          cell={<ActionCell onViewDetails={showDetails} />}
        />
      </DataTable.Table>
      {detailsToShow && (
        <EventDialog event={detailsToShow} onClose={closeDetails} />
      )}
    </React.Fragment>
  );
}

const CustomStyledPanel = styled(StyledPanel)`
  ${StyledButtons} {
    margin-left: ${props => `${props.theme.space[3]}px`};
  }
`;

type EventListState = {
  searchableProps: string[];
  colSortDirs: Record<string, string>;
  detailsToShow?: Event;
};

type EventListProps = {
  search: string;
  events: Event[];
  onSearchChange: (value: string) => void;
};
