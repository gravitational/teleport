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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';
import TeleportContext from 'teleport/teleportContext';
import { makeUserContext } from 'teleport/services/user';
import { LocalNotificationKind } from 'teleport/services/notifications';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';

import { TopBar } from './TopBar';

export default {
  title: 'Teleport/TopBar',
  args: { userContext: true },
};

export function Story() {
  const ctx = new TeleportContext();

  ctx.storeUser.state = makeUserContext({
    userName: 'admin',
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });

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
}
Story.storyName = 'TopBar';

export function TopBarWithNotifications() {
  const ctx = new TeleportContext();

  ctx.storeUser.state = makeUserContext({
    userName: 'admin',
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });
  ctx.storeNotifications.state = {
    notifications: [
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'banana',
          route: '',
        },
        id: '111',
        date: new Date(),
      },
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'apple',
          route: '',
        },
        id: '222',
        date: new Date(),
      },
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '333',
        date: new Date(),
      },
    ],
  };

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
}
