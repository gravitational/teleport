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
import { storiesOf } from '@storybook/react';
import AuditSessions from './AuditSessions';
import Event from 'teleport/services/events/event';

storiesOf('Teleport/Audit', module).add('AuditSessions', () => (
  <AuditSessions {...defaultProps} />
));

const events = [
  {
    code: 'T2004I',
    ei: 10,
    event: 'session.end',
    namespace: 'default',
    sid: '9febab45-6491-11e9-80a1-427cfde50f5a',
    time: '2019-04-22T00:00:51.543Z',
    uid: '6bf836ee-197c-453e-98e5-31511935f22a',
    user: 'admin@example.com',
  },
  {
    code: 'T1000I',
    event: 'user.login',
    method: 'local',
    success: true,
    time: '2019-04-22T00:49:03Z',
    uid: '173d6b6e-d613-44be-8ff6-f9f893791ef2',
    user: 'admin@example.com',
  },
  {
    code: 'T1000I',
    event: 'user.login',
    method: 'local',
    success: true,
    time: '2019-04-22T00:49:03Z',
    uid: '173d6b6e-d613-44be-8ff6-1165258647',
    user: 'admin@example.com',
  },
  {
    code: 'T1000I',
    event: 'user.login',
    method: 'local',
    success: true,
    time: '2019-04-22T00:49:03Z',
    uid: '173d6b6e-d613-44be-8ff6-1900023399',
    user: 'admin@example.com',
  },
];

const defaultProps = {
  events: events.map(e => new Event(e)),
  pageSize: 3,
};
