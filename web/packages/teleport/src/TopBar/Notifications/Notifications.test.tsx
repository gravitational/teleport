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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import {
  StoreNotifications,
  NotificationKind,
} from 'teleport/stores/storeNotifications';

import { Notifications } from './Notifications';

beforeAll(() => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date('2023-01-20'));
});

afterAll(() => {
  jest.useRealTimers();
});

test('due dates and overdue dates', async () => {
  const ctx = createTeleportContext();
  const store = new StoreNotifications();

  store.setNotifications([
    {
      item: {
        kind: NotificationKind.AccessList,
        resourceName: 'carrot',
        route: '',
      },
      id: '1',
      date: new Date('2023-01-25'),
    },
    // overdue 10 days
    {
      item: {
        kind: NotificationKind.AccessList,
        resourceName: 'carrot',
        route: '',
      },
      id: '2',
      date: new Date('2023-01-10'),
    },
    // overdue month
    {
      item: {
        kind: NotificationKind.AccessList,
        resourceName: 'carrot',
        route: '',
      },
      id: '3',
      date: new Date('2022-12-20'),
    },
  ]);

  ctx.storeNotifications = store;

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Notifications />
      </ContextProvider>
    </MemoryRouter>
  );

  // no need to click on button for render.
  // it's already in the dom but hidden.

  expect(screen.queryAllByTestId('note-item')).toHaveLength(3);

  expect(
    screen.getByText(/overdue for a review 10 days ago/i)
  ).toBeInTheDocument();

  expect(
    screen.getByText(/overdue for a review about 1 month ago/i)
  ).toBeInTheDocument();

  expect(
    screen.getByText(/needs your review within 5 days/i)
  ).toBeInTheDocument();
});

test('no notes', async () => {
  const ctx = createTeleportContext();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Notifications />
      </ContextProvider>
    </MemoryRouter>
  );

  // no need to click on button for render.
  // it's already in the dom but hidden.

  expect(screen.queryByTestId('note-item')).not.toBeInTheDocument();
  expect(screen.getByText(/no notifications/i)).toBeInTheDocument();
});
