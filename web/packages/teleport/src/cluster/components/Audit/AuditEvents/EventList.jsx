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
import {
  Table,
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
} from 'design/DataTable';
import EventTypeCell from './EventTypeCell';
import EventDescCell from './EventDescCell';
import { ActionCell, TimeCell } from './EventListCells';
import EventDialog from './../EventDialog';

import { usePages, Pager, StyledPanel } from 'design/DataTable/Paged';

export default function EventList(props) {
  const { events = [], search = '' } = props;

  const [state, setState] = React.useState(() => {
    return {
      searchableProps: ['codeDesc', 'message', 'user', 'time'],
      detailsToShow: null,
      colSortDirs: {
        time: SortTypes.ASC,
      },
    };
  });

  function onSortChange(columnKey, sortDir) {
    setState({
      ...state,
      colSortDirs: { [columnKey]: sortDir },
    });
  }

  function showDetails(detailsToShow) {
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

  const data = React.useMemo(() => {
    const { colSortDirs, searchableProps } = state;
    const filtered = events.filter(obj =>
      isMatch(obj, search, {
        searchableProps: searchableProps,
      })
    );

    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    const sorted = sortBy(filtered, columnKey);
    if (sortDir === SortTypes.ASC) {
      return sorted.reverse();
    }

    return sorted;
  }, [state, events, search]);

  const { detailsToShow, colSortDirs } = state;
  const paged = usePages({ pageSize: 20, data });

  return (
    <React.Fragment>
      {paged.hasPages && (
        <StyledPanel>
          <Pager {...paged} />
        </StyledPanel>
      )}
      <Table data={paged.data}>
        <Column
          columnKey="codeDesc"
          cell={<EventTypeCell />}
          header={
            <SortHeaderCell
              sortDir={colSortDirs.codeDesc}
              onSortChange={onSortChange}
              title="Type"
            />
          }
        />
        <Column
          columnKey="message"
          header={<Cell>Description</Cell>}
          cell={<EventDescCell style={{ wordBreak: 'break-all' }} />}
        />
        <Column
          columnKey="user"
          header={
            <SortHeaderCell
              sortDir={colSortDirs.user}
              onSortChange={onSortChange}
              title="User"
            />
          }
          cell={<TextCell />}
        />
        <Column
          columnKey="time"
          header={
            <SortHeaderCell
              sortDir={colSortDirs.time}
              onSortChange={onSortChange}
              title="Created"
            />
          }
          cell={<TimeCell />}
        />
        <Column
          header={<Cell />}
          cell={<ActionCell onViewDetails={showDetails} />}
        />
      </Table>
      {detailsToShow && (
        <EventDialog event={detailsToShow} onClose={closeDetails} />
      )}
    </React.Fragment>
  );
}
