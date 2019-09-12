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

import $ from 'jquery';
import React from 'react';
import { storiesOf } from '@storybook/react';
import { Audit } from './Audit';
import { getRangeOptions } from './RangePicker';
import Event from 'gravity/cluster/services/events/event';

storiesOf('Gravity/Audit', module)
  .add('Audit', () => (
    <Audit {...props} />
  ))
  .add('Overflow', () => {
    const store = {
      ...props.store,
      overflow: true
    }
    return (
      <Audit {...props } store={store} />
    )}
  )
  .add('Processing', () => {
    return (
      <Audit {...props } attempt={{isProcessing: true}} />
  )})
  .add('Server Error', () => (
      <Audit {...props } attempt={{isFailed: true, message: "server error" }} />
  ))

const props = {
  attempt: {},
  attemptActions: {
    do: () => null
  },
  onFetchLatest: () => $.Deferred().resolve(),
  onFetch: () => $.Deferred().resolve(),
  searchValue: '',
  range: {
    isCustom: true,
    from: new Date('2019-03-22T00:00:51.543Z'),
    to: new Date(),
  },
  rangeOptions: getRangeOptions(),
  store: {
    events,
    getEvents: () => events.map(e => new Event(e)),
  }
}

const events = [
  {
    "code": "T2004I",
    "ei": 10,
    "event": "session.end",
    "namespace": "default",
    "sid": "9febab45-6491-11e9-80a1-427cfde50f5a",
    "time": "2019-04-22T00:00:51.543Z",
    "uid": "6bf836ee-197c-453e-98e5-31511935f22a",
    "user": "admin@example.com"
  },
  {
    "code": "T1000I",
    "event": "user.login",
    "method": "local",
    "success": true,
    "time": "2019-04-22T00:49:03Z",
    "uid": "173d6b6e-d613-44be-8ff6-f9f893791ef2",
    "user": "admin@example.com"
  },
 {
    "code": "T3007W",
    "error": "ssh: principal \"fsdfdsf\" not in the set of valid principals for given certificate: [\"root\"]",
    "event": "auth",
    "success": false,
    "time": "2019-04-22T02:09:06Z",
    "uid": "036659d6-fdf7-40a4-aa80-74d6ac73b9c0",
    "user": "admin@example.com"
  }, {
    "code": "T1000W",
    "error": "user(name=\"fsdfsdf\") not found",
    "event": "user.login",
    "method": "local",
    "success": false,
    "time": "2019-04-22T18:06:32Z",
    "uid": "597bf08b-75b2-4dda-a578-e387c5ce9b76",
    "user": "fsdfsdf"
  }, {
    "addr.local": "172.31.28.130:3022",
    "addr.remote": "151.181.228.114:51454",
    "code": "T2000I",
    "ei": 0,
    "event": "session.start",
    "login": "root",
    "namespace": "default",
    "server_id": "de3800ea-69d9-4d72-a108-97e57f8eb393",
    "sid": "56408539-6536-11e9-80a1-427cfde50f5a",
    "size": "80:25",
    "time": "2019-04-22T19:39:26.676Z",
    "uid": "84c07a99-856c-419f-9de5-15560451a116",
    "user": "admin@example.com"
  }, {
    "code": "T2002I",
    "ei": 3,
    "event": "resize",
    "login": "root",
    "namespace": "default",
    "sid": "56408539-6536-11e9-80a1-427cfde50f5a",
    "size": "80:25",
    "time": "2019-04-22T19:39:52.432Z",
    "uid": "917d8108-3617-4273-ab37-7bbf8e7c1ab9",
    "user": "admin@example.com"
  }, {
    "addr.local": "172.31.28.130:3022",
    "addr.remote": "151.181.228.114:51752",
    "code": "T2001I",
    "ei": 4,
    "event": "session.join",
    "login": "root",
    "namespace": "default",
    "server_id": "de3800ea-69d9-4d72-a108-97e57f8eb393",
    "sid": "56408539-6536-11e9-80a1-427cfde50f5a",
    "time": "2019-04-22T19:39:52.434Z",
    "uid": "13d26190-289b-41d4-af67-c8c8b0617ebe",
    "user": "admin@example.com"
  }, {
    "action": "download",
    "addr.local": "172.31.28.130:3022",
    "addr.remote": "127.0.0.1:55594",
    "code": "T3004I",
    "event": "scp",
    "login": "root",
    "namespace": "default",
    "path": "~/fsdfsdfsdfsdfs",
    "time": "2019-04-22T19:41:23Z",
    "uid": "183ca6de-c24b-4f67-854f-163c01245fa1",
    "user": "admin@example.com"
  }, {
    "code": "GE1000I",
    "event": "role.created",
    "name": "admin323232323f",
    "time": "2019-04-22T19:42:37Z",
    "uid": "19f9248d-0c24-4453-88bb-648fe61feff1",
    "user": "admin@example.com"
  }, {
    "code": "GE2000I",
    "event": "role.deleted",
    "name": "admin323232323f",
    "time": "2019-04-22T19:43:30Z",
    "uid": "5244829f-8d7a-4f70-8978-eb3a8f40743a",
    "user": "admin@example.com"
  }, {
    "code": "GE1000I",
    "event": "role.created",
    "name": "admin323232323",
    "time": "2019-04-22T19:44:04Z",
    "uid": "6738a844-b49f-4c05-aa84-dcb92b8d8c36",
    "user": "admin@example.com"
  }, {
    "code": "G1010I",
    "event": "invite.created",
    "name": "fdsfsdf",
    "roles": ["@teleadmin"],
    "time": "2019-04-22T19:50:48Z",
    "uid": "c5379148-7740-4f76-b0ae-3ddb2438d2a4",
    "user": "admin@example.com"
  }, {
    "code": "G1003I",
    "event": "logforwarder.created",
    "name": "forwarder1",
    "time": "2019-04-22T19:55:18Z",
    "uid": "333a9a50-44eb-4f82-9634-7e830d5e3a86",
    "user": "admin@example.com"
  }, {
    "code": "G2003I",
    "event": "logforwarder.delete",
    "name": "forwarder1",
    "time": "2019-04-22T19:55:55Z",
    "uid": "a0e0ba2d-983e-4b2d-a441-3897fa6c9ef2",
    "user": "admin@example.com"
  }
]
