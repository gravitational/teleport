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
import { render, screen, userEvent } from 'design/utils/testing';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';
import TeleportContext, {
  disabledFeatureFlags,
} from 'teleport/teleportContext';
import { makeUserContext } from 'teleport/services/user';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { NotificationKind } from 'teleport/stores/storeNotifications';

import { clusters } from 'teleport/Clusters/fixtures';

import { TopBar } from './TopBar';

let ctx: TeleportContext;

function setup(): void {
  ctx = new TeleportContext();
  jest
    .spyOn(ctx, 'getFeatureFlags')
    .mockReturnValue({ ...disabledFeatureFlags, assist: true });
  ctx.clusterService.fetchClusters = () => Promise.resolve(clusters);

  ctx.assistEnabled = true;
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

  render(getTopBar());
  await screen.findByTestId('tb-note');

  expect(screen.getByTestId('tb-note')).toBeInTheDocument();
  expect(screen.queryByTestId('tb-note-attention')).not.toBeInTheDocument();
});

test('notification bell with notification', async () => {
  setup();
  ctx.storeNotifications.state = {
    notifications: [
      {
        item: {
          kind: NotificationKind.AccessList,
          resourceName: 'banana',
          route: '',
        },
        id: 'abc',
        date: new Date(),
      },
    ],
  };

  render(getTopBar());
  await screen.findByTestId('tb-note');

  expect(screen.getByTestId('tb-note')).toBeInTheDocument();
  expect(screen.getByTestId('tb-note-attention')).toBeInTheDocument();

  // Test clicking and rendering of dropdown.
  expect(screen.getByTestId('tb-note-dropdown')).not.toBeVisible();

  await userEvent.click(screen.getByTestId('tb-note-button'));
  expect(screen.getByTestId('tb-note-dropdown')).toBeVisible();
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
