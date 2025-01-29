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

import { subMinutes, subSeconds } from 'date-fns';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { NotificationSubKind } from 'teleport/services/notifications';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { Notifications } from './Notifications';

test('notification bell with notifications', async () => {
  const ctx = createTeleportContext();

  jest.spyOn(ctx.notificationService, 'fetchNotifications').mockResolvedValue({
    nextKey: '',
    userLastSeenNotification: subMinutes(Date.now(), 12), // 12 minutes ago
    notifications: [
      {
        id: '1',
        title: 'Example notification 1',
        subKind: NotificationSubKind.UserCreatedInformational,
        createdDate: subSeconds(Date.now(), 15), // 15 seconds ago
        clicked: false,
        labels: [
          {
            name: 'text-content',
            value: 'This is the text content of the notification.',
          },
        ],
      },
      {
        id: '2',
        title: 'Example notification 2',
        subKind: NotificationSubKind.UserCreatedInformational,
        createdDate: subSeconds(Date.now(), 50), // 50 seconds ago
        clicked: false,
        labels: [
          {
            name: 'text-content',
            value: 'This is the text content of the notification.',
          },
        ],
      },
    ],
  });

  jest
    .spyOn(ctx.notificationService, 'upsertLastSeenNotificationTime')
    .mockResolvedValue({
      time: new Date(),
    });

  render(renderNotifications(ctx));

  await screen.findByTestId('tb-notifications-badge');

  expect(screen.getByTestId('tb-notifications')).toBeInTheDocument();

  // Expect there to be 2 notifications.
  expect(screen.getByTestId('tb-notifications-badge')).toHaveTextContent('2');
  expect(screen.queryAllByTestId('notification-item')).toHaveLength(2);
});

test('notification bell with no notifications', async () => {
  const ctx = createTeleportContext();
  jest.spyOn(ctx.notificationService, 'fetchNotifications').mockResolvedValue({
    nextKey: '',
    userLastSeenNotification: subMinutes(Date.now(), 12), // 12 minutes ago
    notifications: [],
  });

  jest
    .spyOn(ctx.notificationService, 'upsertLastSeenNotificationTime')
    .mockResolvedValue({
      time: new Date(),
    });

  render(renderNotifications(ctx));

  await screen.findByText(/you currently have no notifications/i);

  expect(screen.queryByTestId('notification-item')).not.toBeInTheDocument();
});

const renderNotifications = (ctx: TeleportContext) => {
  return (
    <Router history={createMemoryHistory()}>
      <LayoutContextProvider>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getOSSFeatures()}>
            <Notifications />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </LayoutContextProvider>
    </Router>
  );
};
