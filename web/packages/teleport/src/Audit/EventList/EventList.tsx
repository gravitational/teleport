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
import * as Table from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import { Event } from 'teleport/services/audit';
import { State } from 'teleport/useAuditEvents';
import { ActionCell, TimeCell, DescCell } from './EventListCells';
import TypeCell from './EventTypeCell';
import EventDialog from '../EventDialog';
import { displayDateTime } from 'shared/services/loc';

export default function EventList(props: Props) {
  const {
    clusterId,
    events = [],
    search = '',
    fetchMore,
    fetchStatus,
    pageSize = 50,
  } = props;
  const [state, setState] = React.useState<EventListState>(() => {
    return {
      searchableProps: ['codeDesc', 'message', 'user', 'time'],
      detailsToShow: null,
      colSortDirs: {
        time: Table.SortTypes.ASC,
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
        cb: (target, search, prop) => {
          if (prop === 'time') {
            return displayDateTime(target).includes(search);
          }
        },
      })
    );

    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    const sorted = sortBy(filtered, columnKey);
    if (sortDir === Table.SortTypes.ASC) {
      return sorted.reverse();
    }

    return sorted;
  }, [state, events, search]);

  // paginate
  const tableProps = { pageSize, data, fetchMore, fetchStatus };
  const { detailsToShow, colSortDirs } = state;
  return (
    <React.Fragment>
      <PagedTable {...tableProps}>
        <Table.Column
          columnKey="codeDesc"
          cell={<TypeCell clusterId={clusterId} />}
          header={
            <Table.SortHeaderCell
              sortDir={colSortDirs.codeDesc}
              onSortChange={onSortChange}
              title="Type"
            />
          }
        />
        <Table.Column
          columnKey="message"
          header={<Table.Cell>Description</Table.Cell>}
          cell={<DescCell />}
        />
        <Table.Column
          columnKey="time"
          header={
            <Table.SortHeaderCell
              sortDir={colSortDirs.time}
              onSortChange={onSortChange}
              title="Created"
            />
          }
          cell={<TimeCell />}
        />
        <Table.Column
          header={<Table.Cell />}
          cell={
            <ActionCell clusterId={clusterId} onViewDetails={showDetails} />
          }
        />
      </PagedTable>
      {detailsToShow && (
        <EventDialog event={detailsToShow} onClose={closeDetails} />
      )}
    </React.Fragment>
  );
}

type EventListState = {
  searchableProps: string[];
  colSortDirs: Record<string, string>;
  detailsToShow?: Event;
};

type Props = {
  clusterId: State['clusterId'];
  search: State['searchValue'];
  events: State['events'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
  pageSize?: number;
};
