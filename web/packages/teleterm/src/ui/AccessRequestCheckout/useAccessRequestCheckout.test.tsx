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

import { act, renderHook, waitFor } from '@testing-library/react';

import {
  makeKube,
  makeRootCluster,
  makeServer,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { mapRequestToKubeNamespaceUri } from '../services/workspacesService/accessRequestsService';
import useAccessRequestCheckout, {
  PendingListKubeClusterWithOriginalItem,
} from './useAccessRequestCheckout';

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

  appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .updateNamespacesForKubeCluster(
      [
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
      ],
      kube1.uri
    );

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

test('after creating an access request, pending requests and specifiable fields are reset', async () => {
  const kube = makeKube();
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
      resource: kube,
    });
  await appContext.workspacesService
    .getWorkspaceAccessRequestsService(rootClusterUri)
    .addOrRemoveResource({
      kind: 'kube',
      resource: kube2,
    });

  let mockedCreateAccessRequestFn = jest.spyOn(
    appContext.tshd,
    'createAccessRequest'
  );

  const wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      {children}
    </MockAppContextProvider>
  );

  let { result } = renderHook(useAccessRequestCheckout, {
    wrapper,
  });

  act(() =>
    result.current.setSelectedReviewers([{ value: 'bob', label: 'bob' }])
  );
  expect(result.current.selectedReviewers).toEqual([
    { value: 'bob', label: 'bob' },
  ]);

  act(() =>
    result.current.setSelectedResourceRequestRoles(['apple', 'banana'])
  );
  expect(result.current.selectedResourceRequestRoles).toEqual([
    'apple',
    'banana',
  ]);

  await act(() =>
    result.current.createRequest({
      suggestedReviewers: result.current.selectedReviewers.map(r => r.value),
    })
  );

  expect(mockedCreateAccessRequestFn).toHaveBeenCalledWith({
    rootClusterUri: '/clusters/teleport-local',
    resourceIds: [
      {
        clusterName: 'teleport-local',
        kind: 'kube_cluster',
        name: kube.name,
        subResourceName: '',
      },
      {
        clusterName: 'teleport-local',
        kind: 'kube_cluster',
        name: kube2.name,
        subResourceName: '',
      },
    ],
    roles: ['apple', 'banana'],
    suggestedReviewers: ['bob'],
  });
  expect(result.current.requestedCount).toBe(2);

  // Call create again, should've cleared reviewers and previous roles.
  mockedCreateAccessRequestFn.mockClear();
  // A successful create would've cleared all selected resources,
  // so we add it back here to allow creating again.
  await waitFor(async () => {
    expect(result.current.pendingAccessRequests).toHaveLength(0);
  });
  await act(() =>
    appContext.workspacesService
      .getWorkspaceAccessRequestsService(rootClusterUri)
      .addOrRemoveResource({
        kind: 'kube',
        resource: kube,
      })
  );

  await act(() =>
    result.current.createRequest({
      suggestedReviewers: result.current.selectedReviewers.map(r => r.value),
    })
  );
  expect(mockedCreateAccessRequestFn).toHaveBeenCalledWith({
    rootClusterUri: '/clusters/teleport-local',
    resourceIds: [
      {
        clusterName: 'teleport-local',
        kind: 'kube_cluster',
        name: kube.name,
        subResourceName: '',
      },
    ],
    // These fields gotten cleared after the first create.
    roles: [],
    suggestedReviewers: [],
  });
});

test('updating kube namespaces', async () => {
  const kube1 = makeKube();

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

  const wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      {children}
    </MockAppContextProvider>
  );

  let { result } = renderHook(useAccessRequestCheckout, {
    wrapper,
  });

  // Fetching resource roles is done from an effect, let's wait for it to avoid errors about state
  // updates not being wrapped in act.
  await waitFor(() => {
    expect(result.current.fetchResourceRolesAttempt.status).toEqual('success');
  });

  const kubeItem: PendingListKubeClusterWithOriginalItem = {
    kind: 'kube_cluster',
    clusterName: 'local',
    id: kube1.name,
    name: kube1.name,
    originalItem: {
      kind: 'kube',
      resource: {
        uri: kube1.uri,
      },
    },
  };

  // Test order of request are retained.
  await act(async () =>
    result.current.updateNamespacesForKubeCluster(
      [
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n3',
          subResourceName: 'n3',
        },
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n2',
          subResourceName: 'n2',
        },
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n1',
          subResourceName: 'n1',
        },
      ],
      kubeItem
    )
  );
  let savedRequestIds = result.current.pendingAccessRequests
    .filter(r => r.kind === 'namespace')
    .map(r => r.subResourceName);
  expect(savedRequestIds).toEqual(['n3', 'n2', 'n1']);

  // Test order of request are retained a second time.
  await act(async () =>
    result.current.updateNamespacesForKubeCluster(
      [
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n2',
          subResourceName: 'n2',
        },
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n1',
          subResourceName: 'n1',
        },
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n3',
          subResourceName: 'n3',
        },
      ],
      kubeItem
    )
  );
  savedRequestIds = result.current.pendingAccessRequests
    .filter(r => r.kind === 'namespace')
    .map(r => r.subResourceName);
  expect(savedRequestIds).toEqual(['n2', 'n1', 'n3']);

  // Test empty request clears request.
  await act(async () =>
    result.current.updateNamespacesForKubeCluster([], kubeItem)
  );
  savedRequestIds = result.current.pendingAccessRequests
    .filter(r => r.kind === 'namespace')
    .map(r => r.subResourceName);
  expect(savedRequestIds).toEqual([]);

  // Test invalid request.
  expect(() => {
    result.current.updateNamespacesForKubeCluster(
      [
        {
          kind: 'namespace',
          clusterName: 'local',
          id: kube1.name,
          name: 'n2',
          subResourceName: 'n2',
        },
        {
          kind: 'namespace',
          clusterName: 'local',
          id: 'some-id-of-different-kube-cluster-which-is-invalid',
          name: 'n1',
          subResourceName: 'n1',
        },
      ],
      kubeItem
    );
  }).toThrow(/the same requested kube cluster can be updated/);
});
