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
      events: events.map(makeEvent),
      startKey: '',
    });

  return render(ctx);
};

export const Overflow = () => {
  const ctx = new Context();
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({
      events: [],
      startKey: 'cause-overflow',
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
    session_start: '2021-07-22T02:11:14.664957198Z',
    session_stop: '2021-07-22T02:30:35.951372322Z',
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
    session_start: '2021-07-22T02:11:14.664957198Z',
    session_stop: '2021-07-22T02:11:35.951372322Z',
  },
  // session_recording is off
  {
    cluster_name: 'im-a-cluster-name',
    code: 'T2004I',
    ei: 3,
    enhanced_recording: false,
    event: 'session.end',
    interactive: true,
    namespace: 'default',
    participants: ['test'],
    server_addr: '192.168.0.103:3022',
    server_hostname: 'im-a-nodename',
    server_id: 'b01d1943-c6fe-4a25-699065c29671',
    session_recording: 'off',
    session_start: '2021-07-27T23:19:58.420469454Z',
    session_stop: '2021-07-27T23:30:05.345820925Z',
    sid: 'd183ca84-dd94-434a-afee5c2c5f38',
    time: '2021-07-27T23:20:05.346Z',
    uid: '162eac0d-dbd6-47ef-f38b032b3027',
    user: 'test',
  },
  // non-interactive
  {
    cluster_name: 'kimlisa.cloud.gravitational.io',
    code: 'T2004I',
    ei: 1,
    enhanced_recording: false,
    event: 'session.end',
    interactive: false,
    login: 'root',
    namespace: 'default',
    participants: ['foo'],
    server_addr: '172.31.30.254:32962',
    server_hostname: 'ip-172-31-30-254',
    server_id: 'd3ddd1f8-b602-488b-00c66e29879f',
    session_start: '2021-05-21T22:53:55.313562027Z',
    session_stop: '2021-05-21T22:54:27.122508023Z',
    sid: '9d92ad96-a45c-4add-463cc7bc48b1',
    time: '2021-05-21T22:54:27.123Z',
    uid: '984ac949-6605-4f0a-e450aa5665f4',
    user: 'foo',
  },
];
