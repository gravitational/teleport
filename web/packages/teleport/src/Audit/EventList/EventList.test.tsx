/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { screen, within } from '@testing-library/react';

import { render } from 'design/utils/testing';

import makeEvent from '../../services/audit/makeEvent';
import EventList from './EventList';

describe('EventList', () => {
  it('should sort events with same timestamp by event index', () => {
    const sameTimestamp = '2025-09-08T21:25:49.265Z';

    const userCreatedEvent = {
      code: 'T1002I',
      event: 'user.create',
      time: sameTimestamp,
      uid: '5dcf76ab-3f31-40a7-8550-bb29ecea1e42',
      user: 'admin',
      ei: 269750, // Lower event index - should be first
      name: 'alice',
    };

    const userDeletedEvent = {
      code: 'T1004I',
      event: 'user.delete',
      time: sameTimestamp,
      uid: 'int64:0',
      user: 'admin',
      ei: 667167, // Higher event index - should be second
      name: 'bob',
    };

    const events = [makeEvent(userCreatedEvent), makeEvent(userDeletedEvent)];

    render(
      <EventList
        events={events}
        fetchMore={() => null}
        fetchStatus=""
        pageSize={50}
      />
    );

    const table = screen.getByRole('table');
    const rows = within(table).getAllByRole('row');

    const firstDataRow = rows[1];
    const secondDataRow = rows[2];
    expect(
      within(firstDataRow).getByText(/User \[alice\] has been created/i)
    ).toBeInTheDocument();
    expect(
      within(secondDataRow).getByText(/User \[bob\] has been deleted/i)
    ).toBeInTheDocument();
  });

  it('should handle events with different timestamps correctly', () => {
    const olderEvent = {
      code: 'T1002I',
      event: 'user.create',
      time: '2025-09-08T21:25:48.000Z',
      uid: 'uid-1',
      user: 'admin',
      ei: 999999,
      name: 'charlie',
    };

    const newerEvent = {
      code: 'T1004I',
      event: 'user.delete',
      time: '2025-09-08T21:25:49.000Z',
      uid: 'uid-2',
      user: 'admin',
      ei: 1,
      name: 'dave',
    };

    const events = [makeEvent(olderEvent), makeEvent(newerEvent)];

    render(
      <EventList
        events={events}
        fetchMore={() => null}
        fetchStatus=""
        pageSize={50}
      />
    );

    const table = screen.getByRole('table');
    const rows = within(table).getAllByRole('row');

    const firstDataRow = rows[1];
    const secondDataRow = rows[2];

    expect(
      within(firstDataRow).getByText(/User \[dave\] has been deleted/i)
    ).toBeInTheDocument();
    expect(
      within(secondDataRow).getByText(/User \[charlie\] has been created/i)
    ).toBeInTheDocument();
  });
});
