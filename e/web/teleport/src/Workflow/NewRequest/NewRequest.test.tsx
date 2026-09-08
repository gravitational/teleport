/* eslint-disable testing-library/no-node-access */
import {
  waitFor,
  waitForElementToBeRemoved,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import selectEvent from 'react-select-event';

import { fireEvent, screen } from 'design/utils/testing';
import { dryRunResponse } from 'shared/components/AccessRequests/fixtures';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import ecfg from 'e-teleport/config';
import TeleportContextE from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import * as Main from 'teleport/Main/Main';
import { makeUnifiedResource } from 'teleport/services/resources/makeUnifiedResource';
import makeUserContext from 'teleport/services/user/makeUserContext';
import * as service from 'teleport/services/userPreferences/userPreferences';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { renderWithMemoryRouter } from 'teleport/test/helpers/router';
import * as userUserContext from 'teleport/User/UserContext';

import NewRequest from './NewRequest';

const defaultAccessRequestsEntitlement = cfg.entitlements.AccessRequests;

const ctx = new TeleportContextE();
let component;

beforeEach(() => {
  ctx.storeUser.setState({ ...userContext });
  jest.spyOn(userUserContext, 'useUser').mockReturnValue({
    preferences: makeDefaultUserPreferences(),
    updatePreferences: () => null,
    updateClusterPinnedResources: () => null,
    getClusterPinnedResources: () => null,
    updateDiscoverResourcePreferences: () => null,
  });

  jest.spyOn(ctx.resourceService, 'fetchUnifiedResources').mockResolvedValue({
    agents: nodesResponse.map(makeUnifiedResource),
    startKey: '',
    totalCount: nodesResponse.length,
  });

  jest
    .spyOn(ctx.workflowService, 'fetchResourceRequestRoles')
    .mockResolvedValue(['access', 'editor', 'auditor']);

  jest
    .spyOn(ctx.resourceService, 'fetchRequestableRoles')
    .mockImplementation(() =>
      Promise.resolve({
        items: [
          { name: 'role-1', description: 'test1' },
          { name: 'role-2', description: 'test2' },
          { name: 'role-3', description: 'test3' },
        ],
        startKey: '',
      })
    );

  jest
    .spyOn(ctx.workflowService, 'createAccessRequest')
    .mockResolvedValue(dryRunResponse);

  jest.spyOn(service, 'getUserClusterPreferences').mockResolvedValue({});
  jest.spyOn(service, 'getUserPreferences').mockResolvedValue(null);
  jest.spyOn(service, 'updateUserClusterPreferences').mockResolvedValue({});
  jest.spyOn(service, 'updateUserPreferences').mockResolvedValue({});

  jest.spyOn(ctx.clusterService, 'fetchClusters').mockResolvedValue([]);

  jest.spyOn(Main, 'useNoMinWidth').mockReturnValue();
  // Overwrites the IntersectionObserver with a mock so that the `useInfiniteScroll` hook always calls the fetching function.

  global.IntersectionObserver = jest.fn(callback => {
    callback(
      [
        {
          // This is the property that triggers the fetch. We need it to be true.
          isIntersecting: true,
          intersectionRatio: null,
          boundingClientRect: null,
          intersectionRect: null,
          rootBounds: null,
          target: null,
          time: null,
        },
      ],
      null
    );
    return {
      observe: jest.fn(),
      unobserve: jest.fn(),
      disconnect: jest.fn(),
      takeRecords: jest.fn(),
      root: null,
      rootMargin: null,
      scrollMargin: null,
      thresholds: null,
    };
  });

  class ResizeObserver {
    observe() {}

    unobserve() {}

    disconnect() {}
  }

  global.ResizeObserver = ResizeObserver;

  component = (
    <ContextProvider ctx={ctx}>
      <InfoGuidePanelProvider>
        <NewRequest />
      </InfoGuidePanelProvider>
    </ContextProvider>
  );
});

function renderComponent() {
  return renderWithMemoryRouter(component, {
    initialEntries: ['/web/cluster/localhost/requests/new'],
  });
}

afterEach(() => {
  jest.resetAllMocks();

  cfg.entitlements.AccessRequests = defaultAccessRequestsEntitlement;
});

test('add and remove a resource from table', async () => {
  renderComponent();
  await screen.findAllByText('node1-addr');

  // Initial render is a resource table so we select roles
  await selectEvent.select(
    within(screen.getByTestId('resource-selector')).getByRole('combobox'),
    'Roles'
  );

  await screen.findByText('Proceed to Request');
  await screen.findByText('role-1');
  let rows = screen.getAllByText(/role-/i);
  expect(rows).toHaveLength(3);

  // No resources selected yet.
  let checkoutFooter = screen.getByTestId('checkout-footer');
  expect(checkoutFooter).toHaveTextContent(/0/);
  expect(screen.queryByText(/clear selections/i)).not.toBeInTheDocument();
  expect(screen.getByText(/proceed to request/i)).toBeDisabled();

  // Add a resource.
  rows = screen.getAllByText(/request access/i);
  expect(rows).toHaveLength(3);
  await userEvent.click(rows[0]);
  checkoutFooter = screen.getByTestId('checkout-footer');
  expect(checkoutFooter).toHaveTextContent(/1/);
  expect(screen.getByText(/proceed to request/i)).toBeEnabled();

  // Add another one.
  rows = screen.getAllByText(/add to request/i);
  await userEvent.click(rows[0]);
  checkoutFooter = screen.getByTestId('checkout-footer');
  expect(checkoutFooter).toHaveTextContent(/2/);

  // Remove a resource using remove button.
  rows = screen.getAllByText(/remove/i);
  expect(rows).toHaveLength(2);
  fireEvent.click(rows[0]);
  checkoutFooter = screen.getByTestId('checkout-footer');
  expect(checkoutFooter).toHaveTextContent(/1/);

  // Remove resource by using clear button.
  fireEvent.click(screen.getByText(/clear selections/i));
  checkoutFooter = screen.getByTestId('checkout-footer');
  expect(checkoutFooter).toHaveTextContent(/0/);
  expect(screen.queryByText(/clear selections/i)).not.toBeInTheDocument();
  expect(screen.getByText(/proceed to request/i)).toBeDisabled();
});

test('clicking on a resource label constructs predicate query', async () => {
  renderComponent();
  await screen.findAllByText('node1-addr');

  // Click on a label.
  fireEvent.click(await screen.findByText(/test: node1/i));
  await expect(screen.findByPlaceholderText(/search/i)).resolves.toHaveValue(
    'labels["test"] == "node1"'
  );

  // Click on another label.
  fireEvent.click(await screen.findByText(/test: node2/i));
  await expect(screen.findByPlaceholderText(/search/i)).resolves.toHaveValue(
    'labels["test"] == "node1" && labels["test"] == "node2"'
  );
});

test('select uses node hostnames in checkout', async () => {
  renderComponent();

  let hostnames = await screen.findAllByText('hostname-node1');
  // only one in the resource list
  expect(hostnames).toHaveLength(1);
  const addButtons = await screen.findAllByText(/request access/i);

  await userEvent.click(addButtons[0]);
  const proceedToRequest = await screen.findByText('Proceed to Request');
  await userEvent.click(proceedToRequest);

  expect(screen.getByText('1 Resource Selected')).toBeInTheDocument();
  hostnames = await screen.findAllByText('hostname-node1');

  // one in list and one in the checkout
  expect(hostnames).toHaveLength(2);
});

test('select all nodes hostnames in checkout', async () => {
  renderComponent();

  let hostnames = await screen.findAllByText(/hostname-node/);
  expect(hostnames).toHaveLength(1);
  const selectAllBtn = screen.getByTestId('select_all');
  await userEvent.click(selectAllBtn);
  const addButtons = screen.getAllByText(/remove from request/i);

  await userEvent.click(addButtons[0]);
  const proceedToRequest = screen.getByText('Proceed to Request');
  await userEvent.click(proceedToRequest);

  expect(screen.getByText('4 Resources Selected')).toBeInTheDocument();
  hostnames = screen.getAllByText('hostname-node1');

  // one in list and one in the checkout
  expect(hostnames).toHaveLength(2);
});

test('adding resources with bulk action properly adds/removes nodes', async () => {
  renderComponent();

  let hostnames = await screen.findAllByText(/hostname-node/);
  expect(hostnames).toHaveLength(1);
  const selectAllBtn = screen.getByTestId('select_all');
  await userEvent.click(selectAllBtn);
  const addButtons = screen.getAllByText(/remove from request/i);

  await userEvent.click(addButtons[0]);
  const proceedToRequest = screen.getByText('Proceed to Request');
  await userEvent.click(proceedToRequest);

  expect(screen.getByText('4 Resources Selected')).toBeInTheDocument();
  hostnames = screen.getAllByText('hostname-node1');

  await userEvent.click(await screen.findByTestId('close-checkout'));
  await userEvent.click(await screen.findByText(/add\/remove from request/i));
  expect(proceedToRequest).toBeDisabled();
  expect(await screen.findByText('Resources Added (0)')).toBeInTheDocument();
});

test('select all buttons work properly', async () => {
  renderComponent();

  await screen.findAllByText('node1-addr');

  expect(screen.getByText('Resources Added (0)')).toBeInTheDocument();
  const selectAll = screen.getByTestId('select_all');
  await userEvent.click(selectAll);

  const addToResource = screen.getByTestId('add_to_resource');
  await userEvent.click(addToResource);
  expect(screen.getByText('Resources Added (4)')).toBeInTheDocument();

  const proceedToRequest = screen.getByText('Proceed to Request');
  await userEvent.click(proceedToRequest);
  expect(screen.getByText('4 Resources Selected')).toBeInTheDocument();
});

test('legacy renders no usage info', async () => {
  jest
    .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
    .mockResolvedValueOnce(mockUsageWithNotLimitReached);

  renderComponent();
  await waitFor(() => {
    expect(screen.queryByTestId('usage-info')).not.toBeInTheDocument();
  });
});

test('enabled and unlimited renders no usage info', async () => {
  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 0 };

  jest
    .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
    .mockResolvedValueOnce(mockUsageWithNotLimitReached);

  renderComponent();
  await waitFor(() => {
    expect(screen.queryByTestId('usage-info')).not.toBeInTheDocument();
  });
});

test('enabled and limited renders usage info', async () => {
  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 30 };

  jest
    .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
    .mockResolvedValueOnce(mockUsageWithNotLimitReached);

  renderComponent();
  await waitFor(() => {
    expect(screen.getByTestId('usage-info')).toBeInTheDocument();
  });

  let usageInfo = screen.getByTestId('usage-info');
  expect(usageInfo).toHaveTextContent(
    `allocation of ${ecfg.oss.entitlements.AccessRequests.limit} access requests per month`
  );
});

test('enabled and limited renders allocation info', async () => {
  const noBillingPermission = {
    ...defaultUserInfo,
    userAcl: {
      billing: { list: false, read: false },
    },
  };
  ctx.storeUser.setState(makeUserContext(noBillingPermission));

  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 30 };

  renderComponent();
  await waitFor(() => {
    expect(screen.getByTestId('usage-info')).toBeInTheDocument();
  });

  let usageInfo = screen.getByTestId('usage-info');
  expect(usageInfo).toHaveTextContent(
    `Your cluster has an allocation of ${ecfg.oss.entitlements.AccessRequests.limit} access requests per month.`
  );
  expect(usageInfo).not.toHaveTextContent(`been created this month`);
});

test('limited: displays upsell link and button when access request limit is reached', async () => {
  ecfg.oss.isEnterprise = true;
  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 1 };

  jest
    .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
    .mockResolvedValueOnce({
      accessRequestUsage: {
        monthlyLimit: 5,
        monthlyUsed: 5,
      },
    });

  renderComponent();
  await waitFor(() => {
    expect(screen.getByTestId('usage-info')).toBeInTheDocument();
  });

  let ctaTexts = screen.getAllByText(/with teleport identity governance/i);
  expect(ctaTexts).toHaveLength(2);

  let upsellLinks = screen.queryAllByRole('link');
  expect(upsellLinks).toHaveLength(2);
  for (const link of upsellLinks) {
    expect(link).toHaveAttribute('href', expect.stringMatching(/upgrade-igs/i));
  }
});

test('adding constrained resources to requests', async () => {
  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 0 };
  renderComponent();

  const awsTileTitle = await screen.findByTitle(
    'https://aws-console.localhost'
  );
  expect(awsTileTitle).toBeInTheDocument();
  // eslint-disable-next-line testing-library/no-node-access
  const awsTile = awsTileTitle.parentElement?.parentElement;
  expect(awsTile).toBeInTheDocument();
  const addButton = await within(awsTile).findByText(/add to request/i);
  expect(addButton).toBeInTheDocument();
  await userEvent.click(addButton);

  const devOpsArnOption = await screen.findByText('123456789012: DevOps');
  expect(devOpsArnOption).toBeInTheDocument();
  await userEvent.click(devOpsArnOption);

  const proceedToRequest = screen.getByText('Proceed to Request');
  expect(proceedToRequest).toBeEnabled();
  await userEvent.click(proceedToRequest);
  expect(screen.getByText('1 Resource Selected')).toBeInTheDocument();
  await screen.findAllByText('aws-console');
  await screen.findAllByText(/123456789012: devops/i);

  const textbox = screen.getByPlaceholderText(/describe your request/i);
  await userEvent.type(textbox, 'some reason');
  await waitFor(() => {
    expect(textbox).toHaveValue('some reason');
  });

  await userEvent.click(
    screen.getByRole('button', { name: /submit request/i })
  );

  expect(ctx.workflowService.createAccessRequest).toHaveBeenCalledWith(
    {
      assumeStartTime: null,
      dryRun: undefined,
      maxDuration: new Date('2024-02-17T02:51:00.000Z'),
      reason: 'some reason',
      requestTTL: new Date('2024-02-17T02:51:00.000Z'),
      requestKind: 1,
      resourceIds: [],
      resourceAccessIds: [
        {
          id: {
            clusterName: 'localhost',
            kind: 'app',
            name: 'aws-console',
            subResourceName: '',
          },
          constraints: {
            aws_console: {
              role_arns: ['arn:aws:iam::123456789012:role/DevOps'],
            },
          },
        },
      ],
      roles: ['access', 'editor', 'auditor'],
      suggestedReviewers: ['bob', 'cat', 'george washington'],
    },
    undefined
  );

  const successMsg = await screen.findByText(/make another request/i);
  expect(successMsg).toBeInTheDocument();
});

test('adding SSH constrained resources to requests', async () => {
  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 0 };
  renderComponent();

  // Wait for the constrained SSH node to render.
  await screen.findByText('hostname-constrained-node');

  // In the NewRequest flow, NodeSshLoginMenu shows "Add to request" as the dropdown label.
  // Find the dropdown button for the constrained node (contains a chevron SVG).
  const addToRequestButtons = await screen.findAllByText(/add to request/i);
  const dropdownButton = addToRequestButtons.find(btn =>
    btn.closest('button')?.querySelector('svg')
  );
  expect(dropdownButton).toBeDefined();
  await userEvent.click(dropdownButton);

  // The dropdown should show "root" as a requestable login with a checkbox.
  const rootOption = await screen.findByText('root');
  expect(rootOption).toBeInTheDocument();
  await userEvent.click(rootOption);

  // Should now show 'Proceed to Request'.
  const proceedToRequest = screen.getByText('Proceed to Request');
  expect(proceedToRequest).toBeEnabled();
  await userEvent.click(proceedToRequest);

  expect(screen.getByText('1 Resource Selected')).toBeInTheDocument();

  // Type a reason and submit.
  const textbox = screen.getByPlaceholderText(/describe your request/i);
  await userEvent.type(textbox, 'need root');
  await waitFor(() => {
    expect(textbox).toHaveValue('need root');
  });

  await userEvent.click(
    screen.getByRole('button', { name: /submit request/i })
  );

  expect(ctx.workflowService.createAccessRequest).toHaveBeenCalledWith(
    expect.objectContaining({
      resourceAccessIds: [
        {
          id: {
            clusterName: 'localhost',
            kind: 'node',
            name: '3',
            subResourceName: '',
          },
          constraints: {
            ssh: {
              logins: ['root'],
            },
          },
        },
      ],
    }),
    undefined
  );

  const successMsg = await screen.findByText(/make another request/i);
  expect(successMsg).toBeInTheDocument();
});

test('created requests specifiable fields are respected on checkout (not overwritten)', async () => {
  ecfg.oss.entitlements.AccessRequests = { enabled: true, limit: 0 };
  renderComponent();

  // Select a resource.
  await screen.findAllByText('node1-addr');
  let addButtons = screen.queryAllByText(/request access/i);
  await userEvent.click(addButtons[0]);

  // Go to checkout.
  let proceedToRequest = screen.getByText('Proceed to Request');
  await userEvent.click(proceedToRequest);
  expect(screen.getByText('1 Resource Selected')).toBeInTheDocument();
  await screen.findAllByText('node1-addr');

  // Type a reason.
  const textbox = screen.getByPlaceholderText(/describe your request/i);
  await userEvent.type(textbox, 'some reason');
  await waitFor(() => {
    expect(textbox).toHaveValue('some reason');
  });

  // Remove a suggested reviewer.
  screen.queryAllByText(/bob/i);
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'bob{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  // Remove some request roles.
  await userEvent.click(screen.getByText(/auditor/i));
  await userEvent.click(screen.getByText(/editor/i));

  // Add a new reviewer.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'alpaca-reviewer{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));
  screen.queryAllByText(/alpaca-reviewer/i);

  await userEvent.click(
    screen.getByRole('button', { name: /submit request/i })
  );

  expect(ctx.workflowService.createAccessRequest).toHaveBeenCalledWith(
    {
      assumeStartTime: null,
      dryRun: undefined,
      maxDuration: new Date('2024-02-17T02:51:00.000Z'),
      reason: 'some reason',
      requestTTL: new Date('2024-02-17T02:51:00.000Z'),
      requestKind: 1,
      resourceIds: [
        {
          clusterName: 'localhost',
          kind: 'node',
          name: '1',
          subResourceName: '',
        },
      ],
      resourceAccessIds: [],
      roles: ['access'],
      suggestedReviewers: ['cat', 'george washington', 'alpaca-reviewer'],
    },
    undefined
  );

  await screen.findByText(/make another request/i);

  // Go back to selecting resources to test that previous
  // specifiable fields have been cleared.
  await userEvent.click(
    screen.getByRole('button', { name: /make another request/i })
  );
  await waitForElementToBeRemoved(() =>
    screen.queryByTestId('request-checkout')
  );
  // Select a resource.
  await screen.findAllByText('node1-addr');
  addButtons = screen.queryAllByText(/request access/i);
  await userEvent.click(addButtons[0]);

  // Go to checkout.
  proceedToRequest = screen.getByText('Proceed to Request');
  await userEvent.click(proceedToRequest);
  expect(screen.getByText('1 Resource Selected')).toBeInTheDocument();
  await screen.findAllByText('node1-addr');

  jest.clearAllMocks();
  await userEvent.click(
    screen.getByRole('button', { name: /submit request/i })
  );

  await screen.findByText(/resources requested successfully/i);

  expect(ctx.workflowService.createAccessRequest).toHaveBeenCalledWith(
    {
      assumeStartTime: null,
      dryRun: undefined,
      maxDuration: new Date('2024-02-17T02:51:00.000Z'),
      reason: '',
      requestKind: 1,
      requestTTL: new Date('2024-02-17T02:51:00.000Z'),
      resourceIds: [
        {
          clusterName: 'localhost',
          kind: 'node',
          name: '1',
          subResourceName: '',
        },
      ],
      resourceAccessIds: [],
      // These fields gotten reset after the first create.
      roles: ['access', 'editor', 'auditor'],
      suggestedReviewers: ['bob', 'cat', 'george washington'],
    },
    undefined
  );
}, 20000);

test('serverside pagination works for roles', async () => {
  const mockFirstPageResponse = {
    items: [
      { name: 'role-1', description: 'test1' },
      { name: 'role-2', description: 'test2' },
    ],
    startKey: 'next-page-key',
  };

  const mockSecondPageResponse = {
    items: [
      { name: 'role-3', description: 'test3' },
      { name: 'role-4', description: 'test4' },
    ],
    startKey: '',
  };

  const fetchRequestableRolesSpy = jest
    .spyOn(ctx.resourceService, 'fetchRequestableRoles')
    .mockResolvedValueOnce(mockFirstPageResponse)
    .mockResolvedValueOnce(mockSecondPageResponse);

  renderComponent();

  await selectEvent.select(
    within(screen.getByTestId('resource-selector')).getByRole('combobox'),
    'Roles'
  );

  await waitFor(() => {
    expect(fetchRequestableRolesSpy).toHaveBeenCalledWith(
      {
        search: '',
        limit: 20,
      },
      ['role-1', 'role-2', 'role-3']
    );
  });

  expect(screen.getByText('role-1')).toBeInTheDocument();
  expect(screen.getByText('role-2')).toBeInTheDocument();

  const nextPageButton = screen.getByTitle('Next page');
  await userEvent.click(nextPageButton);

  // Verify second request uses the startKey from first response
  await waitFor(() => {
    expect(fetchRequestableRolesSpy).toHaveBeenCalledWith(
      {
        search: '',
        limit: 20,
        startKey: 'next-page-key',
      },
      ['role-1', 'role-2', 'role-3']
    );
  });

  expect(screen.getByText('role-3')).toBeInTheDocument();
  expect(screen.getByText('role-4')).toBeInTheDocument();
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

const nodesResponse = [
  {
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '1',
    kind: 'node',
    clusterId: 'one',
    hostname: 'hostname-node1',
    addr: 'node1-addr',
    tags: [
      {
        name: 'test',
        value: 'node1',
      },
    ],
  },
  {
    tunnel: true,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '2',
    kind: 'node',
    clusterId: 'one',
    hostname: 'node2',
    addr: 'node2-addr',
    tags: [
      {
        name: 'test',
        value: 'node2',
      },
    ],
  },
  {
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['ubuntu', 'root'],
    principals: [
      { principalType: 'logins', granted: ['ubuntu'], requestable: ['root'] },
    ],
    id: '3',
    kind: 'node',
    clusterId: 'localhost',
    hostname: 'hostname-constrained-node',
    addr: 'constrained-node-addr',
    tags: [{ name: 'env', value: 'staging' }],
    supportedFeatureIds: [1, 2],
  },
  {
    kind: 'app',
    name: 'aws-console',
    description: '',
    id: 'aws-console',
    uri: 'https://console.aws.amazon.com/ec2/v2/home',
    publicAddr: 'aws-console.localhost',
    fqdn: 'aws-console.localhost',
    clusterId: 'localhost',
    labels: [{ name: 'type', value: 'aws-console' }],
    awsConsole: true,
    awsRoles: [
      {
        name: 'ReadOnlyAccess',
        display: 'ReadOnlyAccess',
        accountId: '123456789012',
        arn: 'arn:aws:iam::123456789012:role/ReadOnlyAccess',
        requiresRequest: false,
      },
      {
        name: 'DevOps',
        display: 'DevOps',
        accountId: '123456789012',
        arn: 'arn:aws:iam::123456789012:role/DevOps',
        requiresRequest: true,
      },
      {
        name: 'Administrator',
        display: 'Administrator',
        accountId: '123456789012',
        arn: 'arn:aws:iam::123456789012:role/Administrator',
        requiresRequest: true,
      },
    ],
    supportedFeatureIds: [1],
  },
];

const mockUsageWithNotLimitReached = {
  accessRequestUsage: {
    monthlyLimit: 5,
    monthlyUsed: 3,
  },
};
