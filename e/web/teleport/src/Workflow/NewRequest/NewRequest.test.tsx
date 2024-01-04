import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, fireEvent } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import { within, cleanup, waitFor } from '@testing-library/react';

import makeUserContext from 'teleport/services/user/makeUserContext';
import * as userUserContext from 'teleport/User/UserContext';

import { makeUnifiedResource } from 'teleport/services/resources/makeUnifiedResource';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import cfg from 'teleport/config';

import * as Main from 'teleport/Main/Main';

import ecfg from 'e-teleport/config';
import TeleportContextE from 'e-teleport/teleportContextE';

import { AccessRequest, makeAccessRequest } from 'e-teleport/services/workflow';

import NewRequest from './NewRequest';

const defaultIsTeamFlag = cfg.isTeam;
const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultIsUsageBasedBillingFlag = cfg.isUsageBasedBilling;
const defaultIgsFlag = cfg.isIgsEnabled;

describe('new request behavior', () => {
  const ctx = new TeleportContextE();
  let Component;

  beforeEach(() => {
    ctx.storeUser.setState({ ...userContext });
    jest.spyOn(userUserContext, 'useUser').mockReturnValue({
      preferences: makeDefaultUserPreferences(),
      updatePreferences: () => null,
      updateClusterPinnedResources: () => null,
      getClusterPinnedResources: () => null,
    });

    jest.spyOn(ctx.resourceService, 'fetchUnifiedResources').mockResolvedValue({
      agents: nodesResponse.map(makeUnifiedResource),
      startKey: '',
      totalCount: nodesResponse.length,
    });

    jest
      .spyOn(ctx.workflowService, 'createAccessRequest')
      .mockResolvedValue(makeAccessRequest({}));

    jest
      .spyOn(Main, 'useContentMinWidthContext')
      .mockReturnValue({ setEnforceMinWidth: () => null });
    // Overwrites the IntersectionObserver with a mock so that the `useInfiniteScroll` hook always calls the fetching function.
    // eslint-disable-next-line jest/prefer-spy-on
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
        thresholds: null,
      };
    });

    class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
    // eslint-disable-next-line jest/prefer-spy-on
    global.ResizeObserver = ResizeObserver;

    Component = (
      <MemoryRouter initialEntries={[`web/cluster/cluster-id/requests/new`]}>
        <ContextProvider ctx={ctx}>
          <NewRequest />
        </ContextProvider>
      </MemoryRouter>
    );
  });

  afterEach(() => {
    cleanup();
    jest.resetAllMocks();

    cfg.isTeam = defaultIsTeamFlag;
    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.isUsageBasedBilling = defaultIsUsageBasedBillingFlag;
    cfg.isIgsEnabled = defaultIgsFlag;
  });

  test('add and remove a resource from table', async () => {
    cfg.isIgsEnabled = true; // skips fetching for usage, not required for this test
    render(Component);
    jest
      .spyOn(ctx.workflowService, 'createAccessRequest')
      .mockResolvedValue(makeAccessRequest({}));

    // Initial render is a resource table so we select roles
    const inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );
    fireEvent.change(inputEl, { target: { value: 'role' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    await waitFor(() => {
      screen.getByText('Proceed to Request');
    });
    let rows = screen.getAllByText(/role-/i);
    expect(rows).toHaveLength(
      userContext.accessCapabilities.requestableRoles.length
    );

    // No resources selected yet.
    let checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter).toHaveTextContent(/0/);
    expect(screen.queryByText(/clear selections/i)).not.toBeInTheDocument();
    expect(screen.getByText(/proceed to request/i)).toBeDisabled();

    // Add a resource.
    rows = screen.getAllByText(/add to request/i);
    expect(rows).toHaveLength(3);
    fireEvent.click(rows[0]);
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter).toHaveTextContent(/1/);
    expect(screen.getByText(/proceed to request/i)).toBeEnabled();

    // Add another one.
    fireEvent.click(rows[1]);
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
    cfg.isIgsEnabled = true; // skips fetching for usage, not required for this test
    render(Component);

    // We will use node to test predicate (it will be same for all other agents).
    const inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );

    fireEvent.change(inputEl, { target: { value: 'resource' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

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

  test('select all uses node hostnames in checkout', async () => {
    cfg.isIgsEnabled = true; // skips fetching for usage, not required for this test
    jest
      .spyOn(ctx.workflowService, 'fetchResourceRequestRoles')
      .mockResolvedValueOnce(['access']);

    jest
      .spyOn(ctx.workflowService, 'createAccessRequest')
      .mockResolvedValueOnce({
        created: new Date(),
        maxDuration: new Date(),
        sessionTTL: new Date(),
      } as AccessRequest);

    render(Component);

    const inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );
    fireEvent.change(inputEl, { target: { value: 'resource' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    let addrs = await screen.findAllByText('node1-addr');
    // addrs should not increment when the checkout window because we display hostname
    /* eslint-disable  jest-dom/prefer-in-document */
    expect(addrs).toHaveLength(1);
    const addButtons = await screen.findAllByText(/add to request/i);

    fireEvent.click(addButtons[0]);
    await waitFor(() => {
      screen.getByText('Proceed to Request').click();
    });

    expect(screen.getByText('1 Resource Selected')).toBeInTheDocument();
    addrs = await screen.findAllByText('node1-addr');

    expect(addrs).toHaveLength(1);
    /* eslint-enable  jest-dom/prefer-in-document */
  });

  test('select all buttons work properly', async () => {
    cfg.isIgsEnabled = true; // skips fetching for usage, not required for this test
    render(Component);
    jest
      .spyOn(ctx.workflowService, 'fetchResourceRequestRoles')
      .mockResolvedValueOnce(['access']);

    jest
      .spyOn(ctx.workflowService, 'createAccessRequest')
      .mockResolvedValueOnce({
        created: new Date(),
        maxDuration: new Date(),
        sessionTTL: new Date(),
      } as AccessRequest);

    const inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );
    fireEvent.change(inputEl, { target: { value: 'resource' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    expect(screen.getByText('Resources Added (0)')).toBeInTheDocument();
    await waitFor(() => {
      screen.getByTestId('select_all').click();
      screen.getByTestId('add_to_resource').click();
    });

    expect(screen.getByText('Resources Added (2)')).toBeInTheDocument();

    await waitFor(() => {
      screen.getByText('Proceed to Request').click();
    });

    expect(screen.getByText('2 Resources Selected')).toBeInTheDocument();
  });

  test('legacy renders no usage info', async () => {
    ecfg.oss.isEnterprise = true;
    ecfg.oss.isUsageBasedBilling = false;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce(mockUsageWithNotLimitReached);

    render(Component);
    await waitFor(() => {
      expect(screen.queryByTestId('usage-info')).not.toBeInTheDocument();
    });
  });

  test('eub with igs enabled renders no usage info', async () => {
    ecfg.oss.isEnterprise = true;
    ecfg.oss.isUsageBasedBilling = true;
    ecfg.oss.isTeam = false;
    ecfg.oss.isIgsEnabled = true;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce(mockUsageWithNotLimitReached);

    render(Component);
    await waitFor(() => {
      expect(screen.queryByTestId('usage-info')).not.toBeInTheDocument();
    });
  });

  test('team renders usage info', async () => {
    ecfg.oss.isEnterprise = true;
    ecfg.oss.isUsageBasedBilling = true;
    ecfg.oss.isTeam = true;
    ecfg.oss.isIgsEnabled = false;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce(mockUsageWithNotLimitReached);

    render(Component);
    await waitFor(() => {
      expect(screen.getByTestId('usage-info')).toBeInTheDocument();
    });

    let usageInfo = screen.getByTestId('usage-info');
    expect(usageInfo).toHaveTextContent(`3 access requests`);
    expect(usageInfo).toHaveTextContent(
      `allocation of 5 access requests per month`
    );
  });

  test('eub WITHOUT igs enabled renders usage info', async () => {
    ecfg.oss.isEnterprise = true;
    ecfg.oss.isUsageBasedBilling = true;
    ecfg.oss.isTeam = false;
    ecfg.oss.isIgsEnabled = false;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce(mockUsageWithNotLimitReached);

    render(Component);
    await waitFor(() => {
      expect(screen.getByTestId('usage-info')).toBeInTheDocument();
    });

    let usageInfo = screen.getByTestId('usage-info');
    expect(usageInfo).toHaveTextContent(`3 access requests`);
    expect(usageInfo).toHaveTextContent(
      `allocation of 5 access requests per month`
    );
  });

  test('team: displays upsell link and button when access request limit is reached', async () => {
    ecfg.oss.isEnterprise = true;
    ecfg.oss.isUsageBasedBilling = true;
    ecfg.oss.isIgsEnabled = false;
    ecfg.oss.isTeam = true;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce({
        trustedDeviceUsage: {
          devicesUsageLimit: 0,
          devicesInUse: 0,
        },
        accessRequestUsage: {
          monthlyLimit: 5,
          monthlyUsed: 5,
        },
      });

    render(Component);
    await waitFor(() => {
      expect(screen.getByTestId('usage-info')).toBeInTheDocument();
    });

    let ctaTexts = screen.getAllByText(/with teleport enterprise/i);
    expect(ctaTexts).toHaveLength(2);
    expect(
      screen.queryByText(/with identity governance/i)
    ).not.toBeInTheDocument();

    let upsellLinks = await screen.findAllByRole('link');
    expect(upsellLinks).toHaveLength(2);
    for (const link of upsellLinks) {
      expect(link).toHaveAttribute(
        'href',
        expect.stringMatching(/upgrade-team/i)
      );
    }
  });

  test('eub without igs: displays upsell link and button when access request limit is reached', async () => {
    ecfg.oss.isEnterprise = true;
    ecfg.oss.isUsageBasedBilling = true;
    ecfg.oss.isIgsEnabled = false;
    ecfg.oss.isTeam = false;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce({
        trustedDeviceUsage: {
          devicesUsageLimit: 0,
          devicesInUse: 0,
        },
        accessRequestUsage: {
          monthlyLimit: 5,
          monthlyUsed: 5,
        },
      });

    render(Component);
    await waitFor(() => {
      expect(screen.getByTestId('usage-info')).toBeInTheDocument();
    });

    const ctaTexts = screen.getAllByText(/with identity governance/i);
    expect(ctaTexts).toHaveLength(2);
    expect(
      screen.queryByText(/with teleport enterprise/i)
    ).not.toBeInTheDocument();

    const upsellLinks = screen.getAllByRole('link');
    expect(upsellLinks).toHaveLength(2);
    for (const link of upsellLinks) {
      expect(link).toHaveAttribute(
        'href',
        expect.stringMatching(/upgrade-igs/i)
      );
    }
  });
});

const userContext = makeUserContext({
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
});

const nodesResponse = [
  {
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '1',
    kind: 'node',
    clusterId: 'one',
    hostname: 'node1',
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
];

const mockUsageWithNotLimitReached = {
  trustedDeviceUsage: {
    devicesUsageLimit: 0,
    devicesInUse: 0,
  },
  accessRequestUsage: {
    monthlyLimit: 5,
    monthlyUsed: 3,
  },
};
