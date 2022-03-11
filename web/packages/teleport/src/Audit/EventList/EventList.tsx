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
import { ButtonBorder } from 'design';
import Table, { Cell } from 'design/DataTable';
import { dateTimeMatcher } from 'design/utils/match';
import { displayDateTime } from 'shared/services/loc';
import { Event } from 'teleport/services/audit';
import { State } from '../useAuditEvents';
import EventDialog from '../EventDialog';
import renderTypeCell from './EventTypeCell';

export default function EventList(props: Props) {
  const {
    clusterId,
    events = [],
    fetchMore,
    fetchStatus,
    pageSize = 50,
  } = props;
  const [detailsToShow, setDetailsToShow] = useState<Event>();
  return (
    <>
      <Table
        data={events}
        columns={[
          {
            key: 'codeDesc',
            headerText: 'Type',
            isSortable: true,
            render: event => renderTypeCell(event, clusterId),
          },
          {
            key: 'message',
            headerText: 'Description',
            render: renderDescCell,
          },
          {
            key: 'time',
            headerText: 'Created',
            isSortable: true,
            render: renderTimeCell,
          },
          {
            altKey: 'show-details-btn',
            render: event => renderActionCell(event, setDetailsToShow),
          },
        ]}
        emptyText={'No Events Found'}
        isSearchable
        searchableProps={['code', 'codeDesc', 'time', 'user', 'message', 'id']}
        customSearchMatchers={[dateTimeMatcher(['time'])]}
        initialSort={{ key: 'time', dir: 'DESC' }}
        pagination={{ pageSize }}
        fetching={{
          onFetchMore: fetchMore,
          fetchStatus,
        }}
      />
      {detailsToShow && (
        <EventDialog
          event={detailsToShow}
          onClose={() => setDetailsToShow(null)}
        />
      )}
    </>
  );
}

export const renderActionCell = (
  event: Event,
  onShowDetails: (e: Event) => void
) => (
  <Cell align="right">
    <ButtonBorder
      size="small"
      onClick={() => onShowDetails(event)}
      width="87px"
    >
      Details
    </ButtonBorder>
  </Cell>
);

export const renderTimeCell = ({ time }: Event) => (
  <Cell style={{ minWidth: '120px' }}>{displayDateTime(time)}</Cell>
);

export function renderDescCell({ message }: Event) {
  return <Cell style={{ wordBreak: 'break-word' }}>{message}</Cell>;
}

type Props = {
  clusterId: State['clusterId'];
  events: State['events'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
  pageSize?: number;
};
