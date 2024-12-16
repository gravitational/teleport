import { MemoryRouter } from 'react-router';
import { renderHook, act } from '@testing-library/react';
import makeUserContext from 'teleport/services/user/makeUserContext';
import { dryRunResponse } from 'shared/components/AccessRequests/fixtures';
import {
  PendingListItem,
  ResourceMap,
  getEmptyResourceState,
} from 'shared/components/AccessRequests/NewRequest';
import { SelectedResource } from 'shared/components/UnifiedResources/UnifiedResources';
import { AppSubKind, PermissionSet } from 'teleport/services/apps';

import TeleportContextE from 'e-teleport/teleportContextE';

import {
  useNewRequest,
  addOrRemoveIdentityCenterAssignments,
} from './useNewRequest';
import { parseResourceIdUri } from './kube';

import type { UnifiedResourceApp } from 'shared/components/UnifiedResources';

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

test('identity center assignments with addSelectedResources', async () => {
  const wrapper = ({ children }) => <MemoryRouter>{children}</MemoryRouter>;
  let { result } = renderHook(() => useNewRequest(ctx), {
    wrapper,
  });

  const unifiedResourceApps: SelectedResource[] = [
    {
      unifiedResourceId: 'account-1',
      resource: accountApps[0],
    },
    {
      unifiedResourceId: 'account-2',
      resource: accountApps[1],
    },
  ];

  let expectedAssignmentIDs = accountApps[0].permissionSets
    .map(ps => ps.assignmentId)
    .concat(accountApps[1].permissionSets.map(ps => ps.assignmentId));

  await act(async () =>
    result.current.addSelectedResources(unifiedResourceApps)
  );
  let savaedRequestIds = Object.keys(
    result.current.addedResources.aws_ic_account_assignment
  );
  expect(savaedRequestIds).toEqual(expectedAssignmentIDs);

  // adding same resources remvoes it from the resource map.
  await act(async () =>
    result.current.addSelectedResources(unifiedResourceApps)
  );
  savaedRequestIds = Object.keys(
    result.current.addedResources.aws_ic_account_assignment
  );
  expect(savaedRequestIds).toEqual([]);
});

test('addOrRemoveIdentityCenterAssignments', async () => {
  const appName = 'account1';
  const permSets: PermissionSet[] = [
    {
      name: 'AdministratorAccess',
      arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-25beafadd65e32d5',
      assignmentId: 'account1--administratoraccess',
    },
    {
      name: 'DataScientist',
      arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-93cddc3f6dae4744',
      assignmentId: 'account1--datascientist',
    },
    {
      name: 'NetworkAdministrator',
      arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-e644c1c111dc9656',
      assignmentId: 'account1--networkadministrator',
    },
  ];
  const expectedAssignments = {
    'account1--administratoraccess': '"AdministratorAccess" on "account1"',
    'account1--datascientist': '"DataScientist" on "account1"',
    'account1--networkadministrator': '"NetworkAdministrator" on "account1"',
  };

  const newResources: ResourceMap = getEmptyResourceState();
  addOrRemoveIdentityCenterAssignments(appName, permSets, newResources);
  expect(newResources.aws_ic_account_assignment).toEqual(expectedAssignments);

  // invoking addOrRemoveIdentityCenterAssignments with same perm sets
  // removes the assignment
  addOrRemoveIdentityCenterAssignments(appName, permSets, newResources);
  expect(newResources.aws_ic_account_assignment).toEqual({});
});

const accountApps: UnifiedResourceApp[] = [
  {
    kind: 'app',
    id: 'account-1',
    name: 'account-1',
    description: '',
    labels: [
      {
        name: 'teleport.dev/origin',
        value: 'aws-identity-center',
      },
    ],
    awsConsole: false,
    addrWithProtocol: 'https://console.aws.amazon.com',
    friendlyName: 'account-1',
    samlApp: false,
    requiresRequest: true,
    subKind: AppSubKind.AwsIcAccount,
    permissionSets: [
      {
        name: 'AdministratorAccess',
        arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-25beafadd65e32d5',
        assignmentId: 'account-1--administratoraccess',
      },
      {
        name: 'DataScientist',
        arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-93cddc3f6dae4744',
        assignmentId: 'account-1--datascientist',
      },
      {
        name: 'NetworkAdministrator',
        arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-e644c1c111dc9656',
        assignmentId: 'account-1--networkadministrator',
      },
    ],
  },
  {
    kind: 'app',
    id: 'account-2',
    name: 'account-2',
    description: '',
    labels: [
      {
        name: 'teleport.dev/origin',
        value: 'aws-identity-center',
      },
    ],
    awsConsole: false,
    addrWithProtocol: 'https://console.aws.amazon.com',
    friendlyName: 'account-2',
    samlApp: false,
    requiresRequest: true,
    subKind: AppSubKind.AwsIcAccount,
    permissionSets: [
      {
        name: 'AdministratorAccess',
        arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-25beafadd65e32d5',
        assignmentId: 'account-2--administratoraccess',
      },
      {
        name: 'DataScientist',
        arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-93cddc3f6dae4744',
        assignmentId: 'account-2--datascientist',
      },
      {
        name: 'NetworkAdministrator',
        arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-e644c1c111dc9656',
        assignmentId: 'account-2--networkadministrator',
      },
    ],
  },
];
