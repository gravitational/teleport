/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import userEvent from '@testing-library/user-event';
import { act } from 'react';

import { render, screen } from 'design/utils/testing';
import { ClientVersionStatus } from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';

import { TshdClient } from 'teleterm/services/tshd';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeAuthSettings,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { AppUpdaterContextProvider } from 'teleterm/ui/AppUpdater';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { ClusterLogin } from './ClusterLogin';

it('keeps the focus on the password field on submission error', async () => {
  const user = userEvent.setup();
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.addRootCluster(cluster);

  jest
    .spyOn(appContext.tshd, 'login')
    .mockResolvedValue(
      new MockedUnaryCall(undefined, new Error('whoops something went wrong'))
    );

  render(
    <MockAppContextProvider appContext={appContext}>
      <AppUpdaterContextProvider>
        <ClusterLogin
          clusterUri={cluster.uri}
          onCancel={() => {}}
          prefill={{ username: 'alice' }}
          reason={undefined}
        />
      </AppUpdaterContextProvider>
    </MockAppContextProvider>
  );

  const passwordField = await screen.findByLabelText('Password');
  expect(passwordField).toHaveFocus();

  await user.type(passwordField, 'foo');
  await user.click(screen.getByText('Sign In'));

  await screen.findByText('whoops something went wrong');
  expect(passwordField).toHaveFocus();
});

it('shows go to updates button in compatibility warning if there are clusters providing updates', async () => {
  const clusterFoo = makeRootCluster({ uri: '/clusters/foo' });
  const clusterBar = makeRootCluster({ uri: '/clusters/bar' });
  const appContext = new MockAppContext();
  appContext.addRootCluster(clusterFoo);
  appContext.addRootCluster(clusterBar);

  jest.spyOn(appContext.tshd, 'getAuthSettings').mockResolvedValue(
    new MockedUnaryCall({
      localAuthEnabled: true,
      authProviders: [],
      hasMessageOfTheDay: false,
      authType: 'local',
      allowPasswordless: false,
      localConnectorName: '',
      clientVersionStatus: ClientVersionStatus.TOO_NEW,
      versions: {
        minClient: '16.0.0-aa',
        client: '17.0.0',
        server: '17.0.0',
      },
    })
  );

  jest
    .spyOn(appContext.mainProcessClient, 'subscribeToAppUpdateEvents')
    .mockImplementation(callback => {
      callback({
        kind: 'update-not-available',
        autoUpdatesStatus: {
          enabled: true,
          source: 'managing-cluster',
          version: '19.0.0',
          options: {
            highestCompatibleVersion: '',
            managingClusterUri: clusterBar.uri,
            unreachableClusters: [],
            clusters: [
              {
                clusterUri: clusterFoo.uri,
                toolsAutoUpdate: true,
                toolsVersion: '19.0.0',
                minToolsVersion: '18.0.0-aa',
                otherCompatibleClusters: [],
              },
              {
                clusterUri: clusterBar.uri,
                toolsAutoUpdate: true,
                toolsVersion: '17.0.0',
                minToolsVersion: '16.0.0-aa',
                otherCompatibleClusters: [],
              },
            ],
          },
        },
      });
      return { cleanup: () => {} };
    });

  render(
    <MockAppContextProvider appContext={appContext}>
      <AppUpdaterContextProvider>
        <ClusterLogin
          clusterUri={clusterFoo.uri}
          onCancel={() => {}}
          prefill={{ username: 'alice' }}
          reason={undefined}
        />
      </AppUpdaterContextProvider>
    </MockAppContextProvider>
  );

  expect(
    await screen.findByText('Detected potentially incompatible version')
  ).toBeVisible();
  expect(
    await screen.findByRole('button', { name: 'Go to Auto Updates' })
  ).toBeVisible();
});

it('shows two separate prompt texts during SSO login', async () => {
  const user = userEvent.setup();
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.addRootCluster(cluster);

  jest.spyOn(appContext.tshd, 'getAuthSettings').mockReturnValue(
    new MockedUnaryCall(
      makeAuthSettings({
        authType: 'github',
        authProviders: [
          { displayName: 'GitHub', name: 'github', type: 'github' },
        ],
      })
    )
  );

  const { resolve: resolveLoginPromise, promise: loginPromise } =
    Promise.withResolvers<ReturnType<TshdClient['login']>>();
  jest
    .spyOn(appContext.tshd, 'login')
    .mockImplementation(async () => loginPromise);

  const { resolve: resolveGetClusterPromise, promise: getClusterPromise } =
    Promise.withResolvers<ReturnType<TshdClient['getCluster']>>();
  jest
    .spyOn(appContext.tshd, 'getCluster')
    .mockImplementation(async () => getClusterPromise);

  render(
    <MockAppContextProvider appContext={appContext}>
      <AppUpdaterContextProvider>
        <ClusterLogin
          clusterUri={cluster.uri}
          onCancel={() => {}}
          prefill={{ username: 'alice' }}
          reason={undefined}
        />
      </AppUpdaterContextProvider>
    </MockAppContextProvider>
  );

  await user.click(await screen.findByText('GitHub'));

  expect(
    screen.getByText(/follow the steps in the browser/)
  ).toBeInTheDocument();

  await act(async () => {
    resolveLoginPromise(new MockedUnaryCall({}));
  });

  expect(screen.getByText(/Login successful/)).toBeInTheDocument();

  await act(async () => {
    // Resolve the promise to avoid leaving a hanging promise around.
    resolveGetClusterPromise(new MockedUnaryCall(cluster));
  });
});
