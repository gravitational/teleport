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

import { wait } from 'shared/utils/wait';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import {
  makeRootCluster,
  makeLoggedInUser,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';

import { DocumentAuthorizeWebSession } from './DocumentAuthorizeWebSession';

export default {
  title: 'Teleterm/DocumentAuthorizeWebSession',
};

const doc: types.DocumentAuthorizeWebSession = {
  uri: '/docs/e2hyt5',
  rootClusterUri: rootClusterUri,
  kind: 'doc.authorize_web_session',
  title: 'Authorize Web Session',
  webSessionRequest: {
    redirectUri: '',
    token: '',
    id: '',
  },
};

export function DeviceNotTrusted() {
  const rootCluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });
  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={rootCluster.uri}>
        <DocumentAuthorizeWebSession doc={doc} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}

export function DeviceTrusted() {
  const rootCluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({ isDeviceTrusted: true }),
  });
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });
  appContext.clustersService.authenticateWebDevice = async () => {
    await wait(2_000);
    return new MockedUnaryCall({
      confirmationToken: {
        id: '123456789',
        token: '7c8e7438-abe1-4cbc-b3e6-bd233bba967c',
      },
    });
  };

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={rootCluster.uri}>
        <DocumentAuthorizeWebSession doc={doc} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
