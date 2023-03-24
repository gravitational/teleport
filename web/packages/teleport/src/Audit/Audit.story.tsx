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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { ContextProvider, Context } from 'teleport';

import Audit from './Audit';
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
    clusterId="im-a-cluster"
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
