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

import { LocalNotificationKind } from 'teleport/services/notifications';

import { Notification, StoreNotifications } from './storeNotifications';

test('get/set/update notifications', async () => {
  const store = new StoreNotifications();

  expect(store.getNotifications()).toStrictEqual([]);
  expect(
    store.hasNotificationsByKind(LocalNotificationKind.AccessList)
  ).toBeFalsy();

  // set some notifications, sorted by earliest date.
  const newerNote: Notification = {
    item: {
      kind: LocalNotificationKind.AccessList,
      resourceName: 'apple',
      route: '',
    },
    id: '111',
    date: new Date('2023-10-04T09:09:22-07:00'),
  };
  const olderNote: Notification = {
    item: {
      kind: LocalNotificationKind.AccessList,
      resourceName: 'banana',
      route: '',
    },
    id: '222',
    date: new Date('2023-10-01T09:09:22-07:00'),
  };

  store.setNotifications([newerNote, olderNote]);
  expect(store.getNotifications()).toStrictEqual([olderNote, newerNote]);

  // Update notes, sorted by earliest date.
  const newestNote: Notification = {
    item: {
      kind: LocalNotificationKind.AccessList,
      resourceName: 'carrot',
      route: '',
    },
    id: '333',
    date: new Date('2023-11-23T09:09:22-07:00'),
  };
  const newestOlderNote: Notification = {
    item: {
      kind: LocalNotificationKind.AccessList,
      resourceName: 'carrot',
      route: '',
    },
    id: '444',
    date: new Date('2023-10-03T09:09:22-07:00'),
  };
  const newestOldestNote: Notification = {
    item: {
      kind: LocalNotificationKind.AccessList,
      resourceName: 'carrot',
      route: '',
    },
    id: '444',
    date: new Date('2023-10-01T09:09:22-07:00'),
  };
  store.setNotifications([newestNote, newestOldestNote, newestOlderNote]);
  expect(store.getNotifications()).toStrictEqual([
    newestOldestNote,
    newestOlderNote,
    newestNote,
  ]);

  // Test has notifications
  expect(
    store.hasNotificationsByKind(LocalNotificationKind.AccessList)
  ).toBeTruthy();
});
