import { MemoryRouter } from 'react-router';
import { renderHook, act } from '@testing-library/react';
import makeUserContext from 'teleport/services/user/makeUserContext';
import { dryRunResponse } from 'shared/components/AccessRequests/fixtures';
import { PendingListItem } from 'shared/components/AccessRequests/NewRequest';

import TeleportContextE from 'e-teleport/teleportContextE';

import { useNewRequest } from './useNewRequest';
import { parseResourceIdUri } from './kube';

const ctx = new TeleportContextE();

beforeEach(() => {
  ctx.storeUser.setState({ ...userContext });

  jest
    .spyOn(ctx.workflowService, 'fetchResourceRequestRoles')
    .mockResolvedValue(['access', 'editor', 'auditor']);

  jest
    .spyOn(ctx.workflowService, 'createAccessRequest')
    .mockResolvedValue(dryRunResponse);
});

afterEach(() => {
  jest.resetAllMocks();
});

test('update kube namespaces', async () => {
  const wrapper = ({ children }) => <MemoryRouter>{children}</MemoryRouter>;

  let { result } = renderHook(() => useNewRequest(ctx), {
    wrapper,
  });

  const kube1 = { name: 'kube1' };

  const kubeItem: PendingListItem = {
    kind: 'kube_cluster',
    clusterName: 'local',
    id: kube1.name,
    name: kube1.name,
  };

  // Test saving namespaces.
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

  let savaedRequestIds = Object.keys(
    result.current.addedResources.namespace
  ).map(uri => parseResourceIdUri(uri).params.subResourceName);
  expect(savaedRequestIds).toEqual(['n3', 'n2', 'n1']);

  // Test empty request clears request.
  await act(async () =>
    result.current.updateNamespacesForKubeCluster([], kubeItem)
  );
  savaedRequestIds = Object.keys(result.current.addedResources.namespace);
  expect(savaedRequestIds).toEqual([]);

  // Test invalid requests
  await expect(async () => {
    await act(async () => {
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
    });
  }).rejects.toThrow();
});

const defaultUserInfo = {
  cluster: {
    name: 'im-a-cluster-name',
    lastConnected: '2020-11-04T19:07:50.693Z',
    connectedText: '2020-11-04 11:07:50',
    status: 'online',
    url: '/web/cluster/im-a-cluster-name',
    authVersion: '5.0.0-dev',
    nodeCount: 1,
    publicURL: 'localhost:3080',
    proxyVersion: '5.0.0-dev',
  },
  accessCapabilities: {
    requestableRoles: ['role-1', 'role-2', 'role-3'],
    suggestedReviewers: ['reviewer1', 'reviewer2'],
  },
  userAcl: {
    billing: {
      list: true,
    },
  },
};

const userContext = makeUserContext(defaultUserInfo);
