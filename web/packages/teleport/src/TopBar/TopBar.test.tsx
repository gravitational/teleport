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

import { act } from '@testing-library/react';
import { subMinutes, subSeconds } from 'date-fns';
import { createMemoryHistory } from 'history';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { Router } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';
import { clusters } from 'teleport/Clusters/fixtures';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { NotificationSubKind } from 'teleport/services/notifications';
import { makeUserContext } from 'teleport/services/user';
import session from 'teleport/services/websession';
import TeleportContext, {
  disabledFeatureFlags,
} from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { TopBar } from './TopBar';

let ctx: TeleportContext;

const mio = mockIntersectionObserver();

beforeEach(() => {
  class ResizeObserver {
    observe() {}

    unobserve() {}

    disconnect() {}
  }

  global.ResizeObserver = ResizeObserver;
  jest.resetAllMocks();
});
beforeEach(() => jest.resetAllMocks());

function setup(): void {
  ctx = new TeleportContext();
  jest.spyOn(ctx, 'getFeatureFlags').mockReturnValue(disabledFeatureFlags);
  ctx.clusterService.fetchClusters = () => Promise.resolve(clusters);

  ctx.storeUser.state = makeUserContext({
    userName: 'admin',
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });

  mockUserContextProviderWith(makeTestUserContext());
}

test('notification bell without notification', async () => {
  setup();

  jest.spyOn(ctx.notificationService, 'fetchNotifications').mockResolvedValue({
    nextKey: '',
    userLastSeenNotification: subMinutes(Date.now(), 12), // 12 minutes ago
    notifications: [],
  });

  render(getTopBar());
  await screen.findByTestId('tb-notifications');

  expect(screen.getByTestId('tb-notifications')).toBeInTheDocument();
  expect(
    screen.queryByTestId('tb-notifications-badge')
  ).not.toBeInTheDocument();
});

test('notification bell with notification', async () => {
  setup();

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
    ],
  });

  jest
    .spyOn(ctx.notificationService, 'upsertLastSeenNotificationTime')
    .mockResolvedValue({
      time: new Date(),
    });

  render(getTopBar());
  await screen.findByTestId('tb-notifications-badge');

  expect(screen.getByTestId('tb-notifications')).toBeInTheDocument();
  expect(screen.getByTestId('tb-notifications-badge')).toHaveTextContent('1');

  // Test clicking and rendering of dropdown.
  expect(screen.getByTestId('tb-notifications-dropdown')).not.toBeVisible();

  act(mio.enterAll);

  await userEvent.click(screen.getByTestId('tb-notifications-button'));
  expect(screen.getByTestId('tb-notifications-dropdown')).toBeVisible();
});

test('warning icon will show if session requires device trust and is not authorized', async () => {
  setup();
  jest.spyOn(session, 'getDeviceTrustRequired').mockImplementation(() => true);

  render(getTopBar());

  // the icon will show in the topbar and the usermenunav dropdown
  expect(screen.getAllByTestId('device-trust-required-icon')).toHaveLength(2);
});

test('authorized icon will show if session is authorized', async () => {
  setup();
  jest.spyOn(session, 'getDeviceTrustRequired').mockImplementation(() => true);
  jest.spyOn(session, 'getIsDeviceTrusted').mockImplementation(() => true);

  render(getTopBar());

  // the icon will show in the topbar and the usermenunav dropdown
  expect(screen.getAllByTestId('device-trusted-icon')).toHaveLength(2);
});

test('authorized icon will show regardless of device requirements', async () => {
  setup();
  jest.spyOn(session, 'getDeviceTrustRequired').mockImplementation(() => false);
  jest.spyOn(session, 'getIsDeviceTrusted').mockImplementation(() => true);

  render(getTopBar());

  // the icon will show in the topbar and the usermenunav dropdown
  expect(screen.getAllByTestId('device-trusted-icon')).toHaveLength(2);
});

test('icon will not show if device trust is not required', async () => {
  setup();
  jest.spyOn(session, 'getDeviceTrustRequired').mockImplementation(() => false);

  render(getTopBar());

  // no icons will be present
  expect(screen.queryByTestId('device-trust-icon')).not.toBeInTheDocument();
});

const getTopBar = () => {
  return (
    <Router history={createMemoryHistory()}>
      <LayoutContextProvider>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getOSSFeatures()}>
            <TopBar />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </LayoutContextProvider>
    </Router>
  );
};
