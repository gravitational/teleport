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

import { renderHook, waitFor } from '@testing-library/react';

import {
  makeRootCluster,
  makeServer,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import useAccessRequestCheckout from './useAccessRequestCheckout';

test('fetching requestable roles for servers uses UUID, not hostname', async () => {
  const server = makeServer();
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootClusterUri, cluster);
  });
  await appContext.workspacesService.setActiveWorkspace(rootClusterUri);
  appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveResource('node', server.name, server.hostname);

  jest.spyOn(appContext.tshd, 'getRequestableRoles');

  const wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      {children}
    </MockAppContextProvider>
  );

  renderHook(useAccessRequestCheckout, { wrapper });

  await waitFor(() =>
    expect(appContext.tshd.getRequestableRoles).toHaveBeenCalledWith({
      clusterUri: rootClusterUri,
      resourceIds: [
        {
          clusterName: 'teleport-local',
          kind: 'node',
          name: '1234abcd-1234-abcd-1234-abcd1234abcd',
          subResourceName: '',
        },
      ],
    })
  );
});
