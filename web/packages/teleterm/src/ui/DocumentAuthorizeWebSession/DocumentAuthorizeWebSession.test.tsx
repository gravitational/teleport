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

import userEvent from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';

import {
  makeLoggedInUser,
  makeRootCluster,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';

import { DocumentAuthorizeWebSession } from './DocumentAuthorizeWebSession';

const doc: types.DocumentAuthorizeWebSession = {
  uri: '/docs/e2hyt5',
  rootClusterUri: rootClusterUri,
  kind: 'doc.authorize_web_session',
  title: 'Authorize Web Session',
  webSessionRequest: {
    redirectUri: '',
    token: '',
    id: '',
    username: 'alice',
  },
};

test('warning is visible and authorize button is disabled when device is not trusted', async () => {
  const rootCluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({ isDeviceTrusted: false }),
  });
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={rootCluster.uri}>
        <DocumentAuthorizeWebSession doc={doc} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  expect(await screen.findByText(/This device is not trusted/)).toBeVisible();
  expect(await screen.findByText(/Authorize Session/)).toBeDisabled();
});

test('warning is visible and authorize button is disabled when requested user is not logged in', async () => {
  const rootCluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({ isDeviceTrusted: true, name: 'bob' }),
  });
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={rootCluster.uri}>
        <DocumentAuthorizeWebSession doc={doc} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  expect(
    await screen.findByText(/Requested user is not logged in/)
  ).toBeVisible();
  expect(await screen.findByText(/Authorize Session/)).toBeDisabled();
});

test('authorizing a session opens its URL and closes document', async () => {
  jest.spyOn(window, 'open').mockImplementation();
  const rootCluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({ isDeviceTrusted: true }),
  });
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });
  appContext.workspacesService.setState(draftState => {
    draftState.workspaces[rootCluster.uri] = {
      localClusterUri: rootCluster.uri,
      documents: [doc],
      location: undefined,
      accessRequests: undefined,
    };
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={rootCluster.uri}>
        <DocumentAuthorizeWebSession doc={doc} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  const button = await screen.findByText(/Authorize Session/);
  await userEvent.click(button);

  expect(await screen.findByText(/Session Authorized/)).toBeVisible();
  expect(window.open).toHaveBeenCalledWith(
    'https://teleport-local:3080/webapi/devices/webconfirm?id=123456789&token=7c8e7438-abe1-4cbc-b3e6-bd233bba967c'
  );
  expect(
    appContext.workspacesService
      .getWorkspaceDocumentService(rootCluster.uri)
      .getDocuments()
  ).toHaveLength(0);
});
