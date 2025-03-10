/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';

import { ButtonBorder, Flex } from 'design';
import Table, { Cell } from 'design/DataTable';
import { dateTimeMatcher } from 'design/utils/match';

import { Event } from 'teleport/services/audit';

import EventDialog from '../EventDialog';
import { State } from '../useAuditEvents';
import renderTypeCell from './EventTypeCell';
import { ViewInPolicyButton } from './ViewInPolicyButton';

export default function EventList(props: Props) {
  const { events = [], fetchMore, fetchStatus, pageSize = 50 } = props;
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
            render: event => renderTypeCell(event),
          },
          {
            key: 'message',
            headerText: 'Description',
            render: renderDescCell,
          },
          {
            key: 'time',
            headerText: 'Created (UTC)',
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
    <Flex gap={2} justifyContent="flex-end">
      <ViewInPolicyButton event={event} />
      <ButtonBorder
        size="small"
        onClick={() => onShowDetails(event)}
        width="87px"
      >
        Details
      </ButtonBorder>
    </Flex>
  </Cell>
);

export const renderTimeCell = ({ time }: Event) => (
  <Cell style={{ minWidth: '120px' }}>{time.toISOString()}</Cell>
);

export function renderDescCell({ message }: Event) {
  return <Cell style={{ wordBreak: 'break-word' }}>{message}</Cell>;
}

type Props = {
  events: State['events'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
  pageSize?: number;
};
