/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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

test('does not show assist popup if hidePopup is true', async () => {
  setup();

  render(getTopBar({ hidePopup: true }));
  await screen.findByTestId('cluster-selector');

  expect(screen.queryByTestId('assistPopup')).not.toBeInTheDocument();
});

test('shows assist popup if hidePopup is absent', async () => {
  setup();

  render(getTopBar({}));
  await screen.findByTestId('cluster-selector');

  expect(screen.getByTestId('assistPopup')).toBeInTheDocument();
});

test('shows assist popup if hidePopup is false', async () => {
  setup();

  render(getTopBar({ hidePopup: false }));
  await screen.findByTestId('cluster-selector');

  expect(screen.getByTestId('assistPopup')).toBeInTheDocument();
});

test('notification bell without notification', async () => {
  setup();

  render(getTopBar({}));
  await screen.findByTestId('cluster-selector');

  expect(screen.getByTestId('tb-note')).toBeInTheDocument();
  expect(screen.queryByTestId('tb-note-attention')).not.toBeInTheDocument();
});

test('notification bell with notification', async () => {
  setup();
  ctx.storeNotifications.state = {
    notices: [
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

  render(getTopBar({}));
  await screen.findByTestId('cluster-selector');

  expect(screen.getByTestId('tb-note')).toBeInTheDocument();
  expect(screen.getByTestId('tb-note-attention')).toBeInTheDocument();

  // Test clicking and rendering of dropdown.
  expect(screen.getByTestId('tb-note-dropdown')).not.toBeVisible();

  await userEvent.click(screen.getByTestId('tb-note-button'));
  expect(screen.getByTestId('tb-note-dropdown')).toBeVisible();
});

const getTopBar = ({ hidePopup = null }: { hidePopup?: boolean }) => {
  return (
    <Router history={createMemoryHistory()}>
      <LayoutContextProvider>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getOSSFeatures()}>
            <TopBar hidePopup={hidePopup} />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </LayoutContextProvider>
    </Router>
  );
};
