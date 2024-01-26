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
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
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
        <LayoutContextProvider>
          <Notifications />
        </LayoutContextProvider>
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
        <LayoutContextProvider>
          <Notifications />
        </LayoutContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  // no need to click on button for render.
  // it's already in the dom but hidden.

  expect(screen.queryByTestId('note-item')).not.toBeInTheDocument();
  expect(screen.getByText(/no notifications/i)).toBeInTheDocument();
});
