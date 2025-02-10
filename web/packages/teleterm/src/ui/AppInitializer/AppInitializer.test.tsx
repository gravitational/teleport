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

import { act, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { render } from 'design/utils/testing';

import Logger, { NullService } from 'teleterm/logger';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { IAppContext } from 'teleterm/ui/types';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import { AppInitializer } from './AppInitializer';

mockIntersectionObserver();
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
      Connect to {props.dialog.clusterUri}
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

  expect(
    await screen.findByText(`Connect to ${previouslyActiveCluster.uri}`)
  ).toBeInTheDocument();

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

  // The previous dialog has been replaced without a user interaction.
  // In the real app, this happens fast enough that the user doesn't see the previous dialog.
  expect(
    await screen.findByText(`Connect to ${deepLinkCluster.uri}`)
  ).toBeInTheDocument();

  // We confirm the current cluster-connect dialog.
  const dialogSuccessButton = await screen.findByRole('button', {
    name: 'Connect to cluster',
  });
  await userEvent.click(dialogSuccessButton);

  // Check if the first activated workspace is the one from the deep link.
  const el = await screen.findByTitle(/Open Profiles/);
  expect(el.title).toContain(deepLinkCluster.name);
});

test.each<{
  name: string;
  action(appContext: IAppContext): Promise<void>;
  expectHasDocumentsToReopen: boolean;
}>([
  {
    name: 'closing documents reopen dialog via close button discards previous documents',
    action: async () => {
      await userEvent.click(await screen.findByTitle('Close'));
    },
    expectHasDocumentsToReopen: false,
  },
  {
    name: 'starting new session in document reopen dialog discards previous documents',
    action: async () => {
      await userEvent.click(
        await screen.findByRole('button', { name: 'Start New Session' })
      );
    },
    expectHasDocumentsToReopen: false,
  },
  {
    name: 'overwriting document reopen dialog with another regular dialog does not discard documents',
    action: async appContext => {
      act(() => {
        appContext.modalsService.openRegularDialog({
          kind: 'change-access-request-kind',
          onConfirm() {},
          onCancel() {},
        });
      });
    },
    expectHasDocumentsToReopen: true,
  },
])('$name', async testCase => {
  const rootCluster = makeRootCluster();
  const appContext = new MockAppContext();
  jest
    .spyOn(appContext.statePersistenceService, 'getWorkspacesState')
    .mockReturnValue({
      rootClusterUri: rootCluster.uri,
      workspaces: {
        [rootCluster.uri]: {
          localClusterUri: rootCluster.uri,
          documents: [
            {
              kind: 'doc.access_requests',
              uri: '/docs/123',
              state: 'browsing',
              clusterUri: rootCluster.uri,
              requestId: '',
              title: 'Access Requests',
            },
          ],
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
      clusters: [rootCluster],
    })
  );

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

  expect(
    await screen.findByText(
      'Do you want to reopen tabs from the previous session?'
    )
  ).toBeInTheDocument();

  await testCase.action(appContext);

  expect(
    appContext.workspacesService.getWorkspace(rootCluster.uri)
      .hasDocumentsToReopen
  ).toBe(testCase.expectHasDocumentsToReopen);
});
