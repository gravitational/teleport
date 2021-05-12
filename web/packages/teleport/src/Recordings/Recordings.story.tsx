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
import { makeEvent } from 'teleport/services/audit';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { Context, ContextProvider } from 'teleport';
import Recordings from './Recordings';

export default {
  title: 'Teleport/Recordings',
};

export const Loaded = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({
      overflow: false,
      events: events.map(makeEvent),
    });

  return render(ctx);
};

export const Overflow = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({
      overflow: true,
      events: [],
    });

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

function render(ctx) {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/audit/events'],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <Recordings />
      </Router>
    </ContextProvider>
  );
}

const events = [
  {
    code: 'T2004I',
    ei: 10,
    event: 'session.end',
    namespace: 'default',
    sid: '426485-6491-11e9-80a1-427cfde50f5a',
    time: '2019-04-22T00:00:51.543Z',
    uid: '6bf836ee-197c-453e-98e5-31511935f22a',
    user: 'admin@example.com',
    participants: ['one', 'two'],
    server_id: 'serverId',
    server_hostname: 'apple-node',
    interactive: true,
  },
  {
    code: 'T2004I',
    ei: 10,
    event: 'session.end',
    namespace: 'default',
    sid: '377875-6491-11e9-80a1-427cfde50f5a',
    time: '2019-04-22T00:00:51.543Z',
    uid: '6bf836ee-197c-453e-98e5-31511935f22a',
    user: 'admin@example.com',
    participants: ['one', 'two'],
    server_id: 'serverId',
    server_hostname: 'peach-node',
    interactive: true,
  },
  // should not be rendered b/c session_recording is off
  {
    code: 'T2004I',
    ei: 10,
    event: 'session.end',
    namespace: 'default',
    sid: '1682c475-04fd-5c4f-8881-7d4d44a3bfb1',
    time: '2019-05-22T00:00:51.543Z',
    uid: '6bf836ee-197c-453e-98e5-31511935f22a',
    user: 'admin@example.com',
    participants: ['one', 'two'],
    server_id: 'serverId',
    server_hostname: 'pear-node',
    interactive: true,
    session_recording: 'off',
  },
];
