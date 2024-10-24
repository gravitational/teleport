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
  makeKube,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { mapRequestToKubeNamespaceUri } from '../services/workspacesService/accessRequestsService';

import useAccessRequestCheckout from './useAccessRequestCheckout';

test('fetching requestable roles for servers uses UUID, not hostname', async () => {
  const server = makeServer();
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootClusterUri, cluster);
  });
  await appContext.workspacesService.setActiveWorkspace(rootClusterUri);
  await appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveResource({ kind: 'server', resource: server });

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

test('fetching requestable roles for a kube_cluster resource without specifying a namespace', async () => {
  const kube = makeKube();
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootClusterUri, cluster);
  });
  await appContext.workspacesService.setActiveWorkspace(rootClusterUri);
  await appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveResource({
      kind: 'kube',
      resource: kube,
    });

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
          kind: 'kube_cluster',
          name: kube.name,
          subResourceName: '',
        },
      ],
    })
  );
});

test(`fetching requestable roles for a kube cluster's namespaces only creates resource IDs for its namespaces`, async () => {
  const kube1 = makeKube();
  const kube2 = makeKube({
    name: 'kube2',
    uri: `${rootClusterUri}/kubes/kube2`,
  });
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootClusterUri, cluster);
  });
  await appContext.workspacesService.setActiveWorkspace(rootClusterUri);
  await appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveResource({
      kind: 'kube',
      resource: kube1,
    });
  await appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveResource({
      kind: 'kube',
      resource: kube2,
    });

  await appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveKubeNamespaces([
      mapRequestToKubeNamespaceUri({
        clusterUri: rootClusterUri,
        id: kube1.name,
        name: 'namespace1',
      }),
      mapRequestToKubeNamespaceUri({
        clusterUri: rootClusterUri,
        id: kube1.name,
        name: 'namespace2',
      }),
    ]);

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
          kind: 'namespace',
          name: kube1.name,
          subResourceName: 'namespace1',
        },
        {
          clusterName: 'teleport-local',
          kind: 'namespace',
          name: kube1.name,
          subResourceName: 'namespace2',
        },
        {
          clusterName: 'teleport-local',
          kind: 'kube_cluster',
          name: kube2.name,
          subResourceName: '',
        },
      ],
    })
  );
});
