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

import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { ContextProvider, Context } from 'teleport';

import { AuditContainer as Audit } from './Audit';
import EventList from './EventList';
import { events, eventsSample } from './fixtures';

export default {
  title: 'Teleport/Audit',
};

export const LoadedSample = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({ events: eventsSample, startKey: '' });

  return render(ctx);
};

export const LoadedFetchMore = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({ events, startKey: 'any-text' });

  return render(ctx);
};

export const Processing = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () => new Promise(() => null);
  return render(ctx);
};

export const Failed = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.reject(new Error('server error'));
  return render(ctx);
};

export const AllPossibleEvents = () => (
  <EventList
    events={events}
    fetchMore={() => null}
    fetchStatus={''}
    pageSize={1000}
  />
);

function render(ctx) {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/audit/events'],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <Audit />
      </Router>
    </ContextProvider>
  );
}
