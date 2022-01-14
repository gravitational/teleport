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
import Table, { Cell } from 'design/DataTableNext';
import { displayDateTime } from 'shared/services/loc';
import { Event } from 'teleport/services/audit';
import { State } from '../useAuditEvents';
import EventDialog from '../EventDialog';
import TypeCell from './EventTypeCell';

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
            render: event => <TypeCell event={event} clusterId={clusterId} />,
          },
          {
            key: 'message',
            headerText: 'Description',
            render: ({ message }) => <DescCell message={message} />,
          },
          {
            key: 'time',
            headerText: 'Created',
            isSortable: true,
            render: ({ time }) => <TimeCell time={time} />,
          },
          {
            altKey: 'show-details-btn',
            render: event => (
              <ActionCell event={event} onShowDetails={setDetailsToShow} />
            ),
          },
        ]}
        emptyText={'No Events Found'}
        isSearchable
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

export const ActionCell = ({
  event,
  onShowDetails,
}: {
  event: Event;
  onShowDetails: (e: Event) => void;
}) => (
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

export const TimeCell = ({ time }: Pick<Event, 'time'>) => (
  <Cell style={{ minWidth: '120px' }}>{displayDateTime(time)}</Cell>
);

export function DescCell({ message }: Pick<Event, 'message'>) {
  return <Cell style={{ wordBreak: 'break-word' }}>{message}</Cell>;
}

type Props = {
  clusterId: State['clusterId'];
  events: State['events'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
  pageSize?: number;
};
