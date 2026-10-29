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

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeAccessRequest as makeTshdAccessRequest,
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
  appContext.workspacesService.addWorkspace(cluster);
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
  appContext.workspacesService.addWorkspace(cluster);
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
  appContext.workspacesService.addWorkspace(cluster);
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
  appContext.workspacesService.addWorkspace(cluster);
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
    rootClusterUri,
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
    resourceAccessIds: [],
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
    rootClusterUri,
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
    resourceAccessIds: [],
  });
});

test('updating kube namespaces', async () => {
  const kube1 = makeKube();

  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootClusterUri, cluster);
  });
  appContext.workspacesService.addWorkspace(cluster);
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

describe('scheduled access requests', () => {
  const START = new Date('2026-03-01T12:00:00.000Z');
  const FOURTEEN_DAYS_MS = 14 * 24 * 60 * 60 * 1000;

  // setUpCheckout renders the hook with one pending role, then opens the
  // checkout so the dry run runs.
  async function setUpCheckout(createAccessRequest: jest.Mock) {
    const cluster = makeRootCluster();
    const appContext = new MockAppContext();
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(rootClusterUri, cluster);
    });
    appContext.workspacesService.addWorkspace(cluster);
    await appContext.workspacesService.setActiveWorkspace(rootClusterUri);
    await appContext.workspacesService
      .getWorkspaceAccessRequestsService(rootClusterUri)
      .addOrRemoveRole('requested-role');

    appContext.tshd.createAccessRequest = createAccessRequest;

    const wrapper = ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        {children}
      </MockAppContextProvider>
    );
    const { result } = renderHook(useAccessRequestCheckout, { wrapper });
    await act(async () => result.current.setShowCheckout(true));
    return { result };
  }

  // scheduledResponse answers a dry run with a confirmed fixed window.
  function scheduledResponse() {
    return jest.fn().mockResolvedValue(
      new MockedUnaryCall({
        request: makeTshdAccessRequest({
          timing: {
            scheduled: {
              start: Timestamp.fromDate(START),
              duration: BigInt(FOURTEEN_DAYS_MS),
            },
          },
        }),
      })
    );
  }

  beforeEach(() => {
    jest.useFakeTimers({ doNotFake: ['setTimeout', 'queueMicrotask'] });
    jest.setSystemTime(START);
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  test('the dry run probes for scheduled timing with the current start and a 14-day duration', async () => {
    // Set up a dry run that confirms timing.
    const createAccessRequest = scheduledResponse();

    const { result } = await setUpCheckout(createAccessRequest);

    // Assert the probe asks for a 14 day window starting now.
    expect(createAccessRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        dryRun: true,
        timing: {
          scheduled: {
            start: Timestamp.fromDate(START),
            duration: BigInt(FOURTEEN_DAYS_MS),
          },
        },
      })
    );

    // Assert the timing reaches shared state, which activates the controls.
    await waitFor(() =>
      expect(result.current.dryRunResponse?.timing?.scheduled).toEqual({
        start: START,
        duration: FOURTEEN_DAYS_MS,
      })
    );
  });

  test('a timing-absent dry run response keeps the legacy submission path', async () => {
    // Set up an Auth that answers the probe without timing.
    const createAccessRequest = jest
      .fn()
      .mockResolvedValue(
        new MockedUnaryCall({ request: makeTshdAccessRequest() })
      );

    const { result } = await setUpCheckout(createAccessRequest);
    await waitFor(() => expect(result.current.dryRunResponse).toBeTruthy());
    createAccessRequest.mockClear();

    // Submit an access end.
    const accessEnd = new Date(START.getTime() + 2 * 60 * 60 * 1000);
    await act(async () =>
      result.current.createRequest({ reason: 'r', maxDuration: accessEnd })
    );

    // Assert no scheduled payload, and the legacy end sent as is.
    expect(createAccessRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        timing: undefined,
        maxDuration: Timestamp.fromDate(accessEnd),
      })
    );
  });

  test('immediate submission keeps the dry-run start and the selected end after dwelling in the form', async () => {
    // Set up a confirmed window.
    const createAccessRequest = scheduledResponse();
    const { result } = await setUpCheckout(createAccessRequest);
    await waitFor(() =>
      expect(result.current.dryRunResponse?.timing?.scheduled).toBeTruthy()
    );
    createAccessRequest.mockClear();

    // Dwell in the form for 10 minutes before submitting.
    act(() => jest.advanceTimersByTime(10 * 60 * 1000));

    // Submit with no explicit start, meaning "Immediately on Approval".
    const accessEnd = new Date(START.getTime() + 3 * 60 * 60 * 1000);
    const pendingExpiry = new Date(START.getTime() + 24 * 60 * 60 * 1000);
    await act(async () =>
      result.current.createRequest({
        reason: 'r',
        maxDuration: accessEnd,
        requestTTL: pendingExpiry,
      })
    );

    // Assert the start is not rebased, the duration is end minus it, and the
    // pending expiry is unchanged.
    expect(createAccessRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        timing: {
          scheduled: {
            start: Timestamp.fromDate(START),
            duration: BigInt(3 * 60 * 60 * 1000),
          },
        },
        requestTtl: Timestamp.fromDate(pendingExpiry),
      })
    );
  });

  test('an explicit future submission sends the selected start and end unchanged', async () => {
    // Set up a confirmed window.
    const createAccessRequest = scheduledResponse();
    const { result } = await setUpCheckout(createAccessRequest);
    await waitFor(() =>
      expect(result.current.dryRunResponse?.timing?.scheduled).toBeTruthy()
    );
    createAccessRequest.mockClear();

    // Submit a window two days out, lasting four hours.
    const futureStart = new Date(START.getTime() + 2 * 24 * 60 * 60 * 1000);
    const accessEnd = new Date(futureStart.getTime() + 4 * 60 * 60 * 1000);
    await act(async () =>
      result.current.createRequest({
        reason: 'r',
        start: futureStart,
        maxDuration: accessEnd,
      })
    );

    // Assert the selected start is used and spans to the selected end.
    expect(createAccessRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        timing: {
          scheduled: {
            start: Timestamp.fromDate(futureStart),
            duration: BigInt(4 * 60 * 60 * 1000),
          },
        },
      })
    );
  });
});
