/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import 'jest-canvas-mock';
import { render } from 'design/utils/testing';
import { screen, act, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { VnetContextProvider } from 'teleterm/ui/Vnet';
import Logger, { NullService } from 'teleterm/logger';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';

import { AppInitializer } from './AppInitializer';

beforeAll(() => {
  Logger.init(new NullService());
});

jest.mock('teleterm/ui/ClusterConnect', () => ({
  ClusterConnect: props => (
    <div
      data-testid="mocked-dialog"
      data-dialog-kind="cluster-connect"
      data-dialog-is-hidden={props.hidden}
    >
      <button onClick={props.dialog.onSuccess}>Connect to cluster</button>
    </div>
  ),
}));

test('activating a workspace via deep link overrides the previously active workspace', async () => {
  // Before closing the app, both clusters were present in the state, with previouslyActiveCluster being active.
  // However, the user clicked a deep link pointing to deepLinkCluster.
  // The app should prioritize the user's intent by activating the workspace for the deep link,
  // rather than reactivating the previously active cluster.
  const previouslyActiveCluster = makeRootCluster({
    uri: '/clusters/teleport-previously-active',
    proxyHost: 'teleport-previously-active:3080',
    name: 'teleport-previously-active',
    connected: false,
  });
  const deepLinkCluster = makeRootCluster({
    uri: '/clusters/teleport-deep-link',
    proxyHost: 'teleport-deep-link:3080',
    name: 'teleport-deep-link',
    connected: false,
  });
  const appContext = new MockAppContext();
  jest
    .spyOn(appContext.statePersistenceService, 'getWorkspacesState')
    .mockReturnValue({
      rootClusterUri: previouslyActiveCluster.uri,
      workspaces: {
        [previouslyActiveCluster.uri]: {
          localClusterUri: previouslyActiveCluster.uri,
          documents: [],
          location: undefined,
        },
        [deepLinkCluster.uri]: {
          localClusterUri: deepLinkCluster.uri,
          documents: [],
          location: undefined,
        },
      },
    });
  appContext.mainProcessClient.configService.set(
    'usageReporting.enabled',
    false
  );
  jest.spyOn(appContext.tshd, 'listRootClusters').mockReturnValue(
    new MockedUnaryCall({
      clusters: [deepLinkCluster, previouslyActiveCluster],
    })
  );
  jest.spyOn(appContext.modalsService, 'openRegularDialog');
  const userInterfaceReady = withPromiseResolver();
  jest
    .spyOn(appContext.mainProcessClient, 'signalUserInterfaceReadiness')
    .mockImplementation(() => userInterfaceReady.resolve());

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <ResourcesContextProvider>
            <AppInitializer />
          </ResourcesContextProvider>
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  // Wait for the app to finish initialization.
  await act(() => userInterfaceReady.promise);
  // Launch a deep link and do not wait for the result.
  act(() => {
    void appContext.deepLinksService.launchDeepLink({
      status: 'success',
      url: {
        host: deepLinkCluster.proxyHost,
        hostname: deepLinkCluster.name,
        port: '1234',
        pathname: '/authenticate_web_device',
        username: deepLinkCluster.loggedInUser.name,
        searchParams: {
          id: '123',
          redirect_uri: '',
          token: 'abc',
        },
      },
    });
  });

  // The cluster-connect dialog should be opened two times.
  // The first one comes from restoring the previous session, but it is
  // immediately canceled and replaced with a dialog to the cluster from
  // the deep link.
  await waitFor(
    () => {
      expect(appContext.modalsService.openRegularDialog).toHaveBeenCalledTimes(
        2
      );
    },
    // A small timeout to prevent potential race conditions.
    { timeout: 10 }
  );
  expect(appContext.modalsService.openRegularDialog).toHaveBeenNthCalledWith(
    1,
    expect.objectContaining({
      kind: 'cluster-connect',
      clusterUri: previouslyActiveCluster.uri,
    })
  );
  expect(appContext.modalsService.openRegularDialog).toHaveBeenNthCalledWith(
    2,
    expect.objectContaining({
      kind: 'cluster-connect',
      clusterUri: deepLinkCluster.uri,
    })
  );

  // We blindly confirm the current cluster-connect dialog.
  const dialogSuccessButton = await screen.findByRole('button', {
    name: 'Connect to cluster',
  });
  await userEvent.click(dialogSuccessButton);

  // Check if the first activated workspace is the one from the deep link.
  expect(await screen.findByTitle(/Current cluster:/)).toBeVisible();
  expect(
    screen.queryByTitle(`Current cluster: ${deepLinkCluster.name}`)
  ).toBeVisible();
});

//TODO(gzdunek): Replace with Promise.withResolvers after upgrading to Node.js 22.
function withPromiseResolver() {
  let resolver: () => void;
  const promise = new Promise<void>(resolve => (resolver = resolve));
  return {
    resolve: resolver,
    promise,
  };
}
