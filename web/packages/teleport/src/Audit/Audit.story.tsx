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

import { MemoryRouter, Route, Routes } from 'react-router';

import { Context, ContextProvider } from 'teleport';
import cfg from 'teleport/config';

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
  ctx.clusterService.fetchClusters = () => Promise.resolve([]);

  return render(ctx);
};

export const LoadedFetchMore = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({ events, startKey: 'any-text' });
  ctx.clusterService.fetchClusters = () => Promise.resolve([]);

  return render(ctx);
};

export const Processing = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () => new Promise(() => null);
  ctx.clusterService.fetchClusters = () => Promise.resolve([]);

  return render(ctx);
};

export const Failed = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.reject(new Error('server error'));
  ctx.clusterService.fetchClusters = () => Promise.resolve([]);

  return render(ctx);
};

export const AllPossibleEvents = () => (
  <EventList
    events={events}
    search=""
    setSearch={() => null}
    setSort={() => null}
    sort={{ dir: 'ASC', fieldName: 'created' }}
  />
);

function render(ctx) {
  return (
    <MemoryRouter initialEntries={['/web/cluster/localhost/audit/events']}>
      <ContextProvider ctx={ctx}>
        <Routes>
          <Route path={cfg.routes.audit} element={<Audit />} />
        </Routes>
      </ContextProvider>
    </MemoryRouter>
  );
}
