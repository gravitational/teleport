/*
Copyright 2019-2020 Gravitational, Inc.

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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';
import TeleportContext from 'teleport/teleportContext';
import { makeUserContext } from 'teleport/services/user';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { NotificationKind } from 'teleport/stores/storeNotifications';

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
          kind: NotificationKind.AccessList,
          resourceName: 'banana',
          route: '',
        },
        id: '111',
        date: new Date(),
      },
      {
        item: {
          kind: NotificationKind.AccessList,
          resourceName: 'apple',
          route: '',
        },
        id: '222',
        date: new Date(),
      },
      {
        item: {
          kind: NotificationKind.AccessList,
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
