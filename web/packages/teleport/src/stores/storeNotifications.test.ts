/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Notice, StoreNotifications } from './storeNotifications';

test('get/set/update notifications', async () => {
  const store = new StoreNotifications();

  expect(store.getNotifications()).toStrictEqual([]);

  // set some notifications, sorted by earliest date.
  const newerNote: Notice = {
    kind: 'access-lists',
    id: '111',
    resourceName: 'apple',
    date: new Date('2023-10-04T09:09:22-07:00'),
    route: '',
  };
  const olderNote: Notice = {
    kind: 'access-lists',
    id: '222',
    resourceName: 'banana',
    date: new Date('2023-10-01T09:09:22-07:00'),
    route: '',
  };

  store.setNotifications([newerNote, olderNote]);
  expect(store.getNotifications()).toStrictEqual([olderNote, newerNote]);

  // Update notes, sorted by earliest date.
  const newestNote: Notice = {
    kind: 'access-lists',
    id: '333',
    resourceName: 'carrot',
    date: new Date('2023-11-23T09:09:22-07:00'),
    route: '',
  };
  const newestOlderNote: Notice = {
    kind: 'access-lists',
    id: '444',
    resourceName: 'carrot',
    date: new Date('2023-10-03T09:09:22-07:00'),
    route: '',
  };
  const newestOldestNote: Notice = {
    kind: 'access-lists',
    id: '444',
    resourceName: 'carrot',
    date: new Date('2023-10-01T09:09:22-07:00'),
    route: '',
  };
  store.setNotifications([newestNote, newestOldestNote, newestOlderNote]);
  expect(store.getNotifications()).toStrictEqual([
    newestOldestNote,
    newestOlderNote,
    newestNote,
  ]);
});
