import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, fireEvent } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import { within, waitFor } from '@testing-library/react';

import makeUserContext from 'teleport/services/user/makeUserContext';
import { Node } from 'teleport/services/nodes/types';

import cfg from 'e-teleport/config';
import TeleportContextE from 'e-teleport/teleportContextE';

import { AccessRequest } from 'e-teleport/services/workflow';

import NewRequest from './NewRequest';

import type { App } from 'teleport/services/apps';

describe('new request behavior', () => {
  const ctx = new TeleportContextE();
  let Component;

  beforeEach(() => {
    ctx.storeUser.setState({ ...userContext });
    jest.spyOn(ctx.nodeService, 'fetchNodes').mockResolvedValue({
      agents: nodes,
      startKey: '',
      totalCount: nodes.length,
    });

    Component = (
      <MemoryRouter initialEntries={[`web/cluster/cluster-id/requests/new`]}>
        <ContextProvider ctx={ctx}>
          <NewRequest />
        </ContextProvider>
      </MemoryRouter>
    );
  });

  test('add and remove a resource from table', async () => {
    render(Component);

    // Initial render is a roles table.
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
    render(Component);

    // We will use node to test predicate (it will be same for all other agents).
    const inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );

    fireEvent.change(inputEl, { target: { value: 'node' } });
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
    jest
      .spyOn(ctx.nodeService, 'fetchNodes')
      .mockResolvedValueOnce({
        agents: nodes,
        startKey: '',
        totalCount: nodes.length,
      })
      .mockResolvedValueOnce({
        agents: nodes,
        startKey: '',
        totalCount: nodes.length,
      });

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
    fireEvent.change(inputEl, { target: { value: 'node' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    await screen.findByText(/node1-addr/i);

    fireEvent.click(screen.getByText('+ Add all'));
    let hostnames = await screen.findAllByText('node1');
    let addrs = await screen.findAllByText('node1-addr');
    // as we aren't checking for existence but checking to make sure only ONE exists now, then 2 exist later
    // (hostnames.length).toBe(1) suggests toHaveLength so either way, we are disabling this line (and subsequent)
    /* eslint-disable  jest-dom/prefer-in-document */
    expect(hostnames).toHaveLength(1);
    expect(addrs).toHaveLength(1);
    await waitFor(() => {
      screen.getByText('Proceed to Request').click();
    });

    expect(screen.getByText('2 Resources Selected')).toBeInTheDocument();
    hostnames = await screen.findAllByText('node1');
    addrs = await screen.findAllByText('node1-addr');

    // we should see the hostname "node1" twice in the doc now, once in the resource list and once in the checkout table
    // and addr only once (not added to checkout table)
    expect(hostnames).toHaveLength(2);
    expect(addrs).toHaveLength(1);

    /* eslint-enable  jest-dom/prefer-in-document */
  });

  test('select all buttons work properly', async () => {
    // The first fetch will only make a request for the first page (10 items).
    // The second fetch will come from the request to fetch all apps across all pages
    // in order to add all of them to the access request.
    jest
      .spyOn(ctx.appService, 'fetchApps')
      .mockResolvedValueOnce({
        agents: appsFirstPage,
        startKey: '',
        totalCount: allApps.length,
      })
      .mockResolvedValueOnce({
        agents: allApps,
        startKey: '',
        totalCount: allApps.length,
      });

    render(Component);

    // Change dropdown selector to show applications.
    const inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );
    fireEvent.change(inputEl, { target: { value: 'app' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    // Wait until the apps are listed
    await screen.findByText(/number: 2/i);

    // Ensure that clicking "+ ADD ALL" in the corner adds the first page (10 items) to the request.
    fireEvent.click(screen.getByText('+ Add all'));
    let checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter).toHaveTextContent('Resources Added (10)');
    // There should be no '+ Add to request' button on any row since all of them should have been added.
    expect(screen.queryByText('+ Add to request')).not.toBeInTheDocument();

    // Ensure that the panel to select all agents across all pages is now visible.
    expect(
      screen.getByText(/10 applications on this page/i)
    ).toBeInTheDocument();

    // Ensure that adding all applications across all pages works properly.
    fireEvent.click(screen.getByText('+ Add all 18 matching applications'));
    expect(ctx.appService.fetchApps).toHaveBeenCalledTimes(2);
    await screen.findByText(/18 applications across 2 pages/i);
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter).toHaveTextContent('Resources Added (18)');

    // Ensure that clearing the selection works properly.
    fireEvent.click(screen.getByText('Remove all 18 matching applications'));
    await screen.findByText('+ Add all');
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter).toHaveTextContent('Resources Added (0)');
  });

  test('user group select', async () => {
    jest.spyOn(ctx.appService, 'fetchApps').mockResolvedValueOnce({
      agents: appsWithUserGroups,
      startKey: '',
      totalCount: allApps.length,
    });

    render(Component);

    // Change dropdown selector to show applications.
    let inputEl = within(screen.getByTestId('resource-selector')).getByRole(
      'textbox'
    );
    fireEvent.change(inputEl, { target: { value: 'app' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    // Wait until the apps are listed
    await screen.findByText(/number: 2/i);

    // Start by adding all apps.
    fireEvent.click(screen.getByText('+ Add all'));
    let checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter).toHaveTextContent(`Resources Added (4)`);
    screen.getByText(/remove 4 applications/i);

    // Test selecting a user group, updates all other rows that have same user group.
    let ugInputs = screen.getAllByText(/alternatively select user groups/i);
    expect(ugInputs).toHaveLength(3);

    // Select a user group from downdown.
    inputEl = ugInputs[0];
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('ug1-description'));

    // The total resource added should be 3 (1 less than total apps)
    // b/c 2 apps share the same user group (1 resource request to shared user group)
    const changedInputs = screen.getAllByText(/1 user groups added/i);
    expect(changedInputs).toHaveLength(2);
    expect(screen.getByTestId('checkout-footer')).toHaveTextContent(
      `Resources Added (3)`
    );
    // Selecting apps by "user groups" don't count towards app selected count.
    screen.getByText(/remove 2 applications/i);

    // Selecting the same user group is as same as deselecting the user group.
    inputEl = changedInputs[0];
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('ug1-description'));

    // Test only selected apps remain.
    expect(screen.getByTestId('checkout-footer')).toHaveTextContent(
      `Resources Added (2)`
    );
    screen.getByText(/remove 2 applications/i);
    screen.getByText(/alternatively select user groups/i);
  });

  test('displays usage info when usage-based billing is used', async () => {
    cfg.oss.isUsageBasedBilling = true;
    jest
      .spyOn(ctx.cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValueOnce({
        trustedDeviceUsage: {
          devicesUsageLimit: 0,
          devicesInUse: 0,
        },
        accessRequestUsage: {
          monthlyLimit: 5,
          monthlyUsed: 3,
        },
      });
    render(Component);

    await waitFor(() => {
      expect(screen.getByTestId('usage-info')).toBeInTheDocument();
    });

    const usageInfo = screen.getByTestId('usage-info');
    expect(usageInfo).toHaveTextContent(`3 access requests`);
    expect(usageInfo).toHaveTextContent(
      `allocation of 5 access requests per month`
    );
  });

  test('displays upsell link and button when access request limit is reached', async () => {
    cfg.oss.isUsageBasedBilling = true;
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

    const usageInfo = screen.getByTestId('usage-info');
    expect(usageInfo).toHaveTextContent(`reached its allocation`);
    const upsellLinks = await screen.findAllByRole('link');
    expect(upsellLinks).toHaveLength(2);
    for (const link of upsellLinks) {
      expect(link).toHaveAttribute(
        'href',
        expect.stringMatching(/https:\/\/goteleport.com\/r\/upgrade/i)
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
});

const nodes: Node[] = [
  {
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '1',
    kind: 'node',
    clusterId: 'one',
    hostname: 'node1',
    addr: 'node1-addr',
    labels: [
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
    labels: [
      {
        name: 'test',
        value: 'node2',
      },
    ],
  },
];

const appsFirstPage: App[] = [
  {
    id: 'dummy-3834740555',
    kind: 'app',
    name: 'dummy-746715940',
    uri: 'http://kizuud.im/het',
    publicAddr: 'http://div.az/busihuj',
    launchUrl: 'http://ramufica.sk/teba',
    awsRoles: [],
    description: 'This is dummy-2899140136 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '1' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://olfuptad.zw/zecco',
    userGroups: [],
  },
  {
    id: 'dummy-735596623',
    kind: 'app',
    name: 'dummy-3881558958',
    uri: 'http://ima.sd/ivdijwah',
    publicAddr: 'http://kav.cl/zi',
    launchUrl: 'http://woijo.to/anogucfac',
    awsRoles: [],
    description: 'This is dummy-396777662 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '2' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://tefe.sg/kis',
    userGroups: [],
  },
  {
    id: 'dummy-61474169',
    kind: 'app',
    name: 'dummy-2813408209',
    uri: 'http://borrepci.mk/cunnenoc',
    publicAddr: 'http://luwop.km/pagmot',
    launchUrl: 'http://emim.cv/nasdeasa',
    awsRoles: [],
    description: 'This is dummy-1292837400 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '3' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://sudlinot.hn/jomevo',
    userGroups: [],
  },
  {
    id: 'dummy-19793371',
    kind: 'app',
    name: 'dummy-2395663666',
    uri: 'http://mipduzok.ml/figinal',
    publicAddr: 'http://zo.in/kasarot',
    launchUrl: 'http://fawkifivi.eh/tuwne',
    awsRoles: [],
    description: 'This is dummy-1084121588 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '4' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://efijena.ae/lehgofuj',
    userGroups: [],
  },
  {
    id: 'dummy-526000410',
    kind: 'app',
    name: 'dummy-1390659234',
    uri: 'http://sus.pm/pumpul',
    publicAddr: 'http://ofu.pw/wesa',
    launchUrl: 'http://wekhe.es/episu',
    awsRoles: [],
    description: 'This is dummy-1282159128 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '5' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://lamice.gh/ifi',
    userGroups: [],
  },
  {
    id: 'dummy-2992909050',
    kind: 'app',
    name: 'dummy-88922302',
    uri: 'http://zad.br/cita',
    publicAddr: 'http://sothaki.gov/evgos',
    launchUrl: 'http://lepeh.sl/dadgac',
    awsRoles: [],
    description: 'This is dummy-786788850 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '6' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://ruswuzup.ua/apcafuc',
    userGroups: [],
  },
  {
    id: 'dummy-886097749',
    kind: 'app',
    name: 'dummy-2613789701',
    uri: 'http://zuftukuw.sb/jiducdo',
    publicAddr: 'http://fu.ro/cehbiucu',
    launchUrl: 'http://akzuvofi.ca/gif',
    awsRoles: [],
    description: 'This is dummy-1640056423 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '7' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://ra.mn/bowiguj',
    userGroups: [],
  },
  {
    id: 'dummy-2279168647',
    kind: 'app',
    name: 'dummy-127659030',
    uri: 'http://ugnob.gr/wi',
    publicAddr: 'http://aru.cd/adeficap',
    launchUrl: 'http://armi.ht/jusmufaf',
    awsRoles: [],
    description: 'This is dummy-2620747129 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '8' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://deufdoc.gq/fur',
    userGroups: [],
  },
  {
    id: 'dummy-3694860682',
    kind: 'app',
    name: 'dummy-3936077543',
    uri: 'http://uvu.ar/fezok',
    publicAddr: 'http://siffe.nc/buvu',
    launchUrl: 'http://ziz.cv/nefag',
    awsRoles: [],
    description: 'This is dummy-2000000720 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '9' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://lebup.ki/sopsupehi',
    userGroups: [],
  },
  {
    id: 'dummy-2592544769',
    kind: 'app',
    name: 'dummy-2726596285',
    uri: 'http://kofare.net/bilishow',
    publicAddr: 'http://fuzejad.bi/suvecisiz',
    launchUrl: 'http://tuvvuskoc.nf/sinugju',
    awsRoles: [],
    description: 'This is dummy-2657854175 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '10' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://dulzawu.vc/ji',
    userGroups: [],
  },
];

const allApps: App[] = [
  {
    id: 'dummy-3834740555',
    kind: 'app',
    name: 'dummy-746715940',
    uri: 'http://kizuud.im/het',
    publicAddr: 'http://div.az/busihuj',
    launchUrl: 'http://ramufica.sk/teba',
    awsRoles: [],
    description: 'This is dummy-2899140136 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '1' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://olfuptad.zw/zecco',
    userGroups: [],
  },
  {
    id: 'dummy-735596623',
    kind: 'app',
    name: 'dummy-3881558958',
    uri: 'http://ima.sd/ivdijwah',
    publicAddr: 'http://kav.cl/zi',
    launchUrl: 'http://woijo.to/anogucfac',
    awsRoles: [],
    description: 'This is dummy-396777662 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '2' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://tefe.sg/kis',
    userGroups: [],
  },
  {
    id: 'dummy-61474169',
    kind: 'app',
    name: 'dummy-2813408209',
    uri: 'http://borrepci.mk/cunnenoc',
    publicAddr: 'http://luwop.km/pagmot',
    launchUrl: 'http://emim.cv/nasdeasa',
    awsRoles: [],
    description: 'This is dummy-1292837400 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '3' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://sudlinot.hn/jomevo',
    userGroups: [],
  },
  {
    id: 'dummy-19793371',
    kind: 'app',
    name: 'dummy-2395663666',
    uri: 'http://mipduzok.ml/figinal',
    publicAddr: 'http://zo.in/kasarot',
    launchUrl: 'http://fawkifivi.eh/tuwne',
    awsRoles: [],
    description: 'This is dummy-1084121588 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '4' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://efijena.ae/lehgofuj',
    userGroups: [],
  },
  {
    id: 'dummy-526000410',
    kind: 'app',
    name: 'dummy-1390659234',
    uri: 'http://sus.pm/pumpul',
    publicAddr: 'http://ofu.pw/wesa',
    launchUrl: 'http://wekhe.es/episu',
    awsRoles: [],
    description: 'This is dummy-1282159128 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '5' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://lamice.gh/ifi',
    userGroups: [],
  },
  {
    id: 'dummy-2992909050',
    kind: 'app',
    name: 'dummy-88922302',
    uri: 'http://zad.br/cita',
    publicAddr: 'http://sothaki.gov/evgos',
    launchUrl: 'http://lepeh.sl/dadgac',
    awsRoles: [],
    description: 'This is dummy-786788850 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '6' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://ruswuzup.ua/apcafuc',
    userGroups: [],
  },
  {
    id: 'dummy-886097749',
    kind: 'app',
    name: 'dummy-2613789701',
    uri: 'http://zuftukuw.sb/jiducdo',
    publicAddr: 'http://fu.ro/cehbiucu',
    launchUrl: 'http://akzuvofi.ca/gif',
    awsRoles: [],
    description: 'This is dummy-1640056423 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '7' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://ra.mn/bowiguj',
    userGroups: [],
  },
  {
    id: 'dummy-2279168647',
    kind: 'app',
    name: 'dummy-127659030',
    uri: 'http://ugnob.gr/wi',
    publicAddr: 'http://aru.cd/adeficap',
    launchUrl: 'http://armi.ht/jusmufaf',
    awsRoles: [],
    description: 'This is dummy-2620747129 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '8' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://deufdoc.gq/fur',
    userGroups: [],
  },
  {
    id: 'dummy-3694860682',
    kind: 'app',
    name: 'dummy-3936077543',
    uri: 'http://uvu.ar/fezok',
    publicAddr: 'http://siffe.nc/buvu',
    launchUrl: 'http://ziz.cv/nefag',
    awsRoles: [],
    description: 'This is dummy-2000000720 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '9' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://lebup.ki/sopsupehi',
    userGroups: [],
  },
  {
    id: 'dummy-2592544769',
    kind: 'app',
    name: 'dummy-2726596285',
    uri: 'http://kofare.net/bilishow',
    publicAddr: 'http://fuzejad.bi/suvecisiz',
    launchUrl: 'http://tuvvuskoc.nf/sinugju',
    awsRoles: [],
    description: 'This is dummy-2657854175 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '10' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://dulzawu.vc/ji',
    userGroups: [],
  },
  {
    id: 'dummy-2070848380',
    kind: 'app',
    name: 'dummy-1656249156',
    uri: 'http://kismucle.sc/zipikpa',
    publicAddr: 'http://utofodil.bb/josug',
    launchUrl: 'http://omjad.ss/oca',
    awsRoles: [],
    description: 'This is dummy-1543743690 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '11' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://fo.be/mohpacev',
    userGroups: [],
  },
  {
    id: 'dummy-1041383074',
    kind: 'app',
    name: 'dummy-3538024861',
    uri: 'http://hawisi.yt/moiki',
    publicAddr: 'http://ni.hm/gilemte',
    launchUrl: 'http://ruki.tj/vid',
    awsRoles: [],
    description: 'This is dummy-2009353871 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '12' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://tujfo.aw/jecusnow',
    userGroups: [],
  },
  {
    id: 'dummy-310160582',
    kind: 'app',
    name: 'dummy-3473759171',
    uri: 'http://ep.cv/urahec',
    publicAddr: 'http://wocim.dz/johhuk',
    launchUrl: 'http://litvojhul.nf/vezidote',
    awsRoles: [],
    description: 'This is dummy-2996867063 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '13' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://rahi.vu/dik',
    userGroups: [],
  },
  {
    id: 'dummy-531607002',
    kind: 'app',
    name: 'dummy-1290775270',
    uri: 'http://lenet.re/beshoize',
    publicAddr: 'http://waiduroh.br/kar',
    launchUrl: 'http://zifewu.iq/bajwogik',
    awsRoles: [],
    description: 'This is dummy-1838805940 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '14' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://bu.sz/heko',
    userGroups: [],
  },
  {
    id: 'dummy-1649180430',
    kind: 'app',
    name: 'dummy-3998870487',
    uri: 'http://wer.za/jegnezes',
    publicAddr: 'http://jihkogsi.om/takzogu',
    launchUrl: 'http://tekla.sk/colipocu',
    awsRoles: [],
    description: 'This is dummy-783255194 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '15' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://hodtatgu.ms/zeduf',
    userGroups: [],
  },
  {
    id: 'dummy-393072968',
    kind: 'app',
    name: 'dummy-1165467974',
    uri: 'http://voholju.cn/otru',
    publicAddr: 'http://asmetme.ai/pawpagur',
    launchUrl: 'http://acalu.pg/onmuke',
    awsRoles: [],
    description: 'This is dummy-1540764346 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '16' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://uhawo.pg/it',
    userGroups: [],
  },
  {
    id: 'dummy-2826879942',
    kind: 'app',
    name: 'dummy-1336558313',
    uri: 'http://ka.ni/wut',
    publicAddr: 'http://jorge.rs/li',
    launchUrl: 'http://ziah.ao/mad',
    awsRoles: [],
    description: 'This is dummy-3882585031 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '17' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://fe.rw/ejaesogeb',
    userGroups: [],
  },
  {
    id: 'dummy-3147977584',
    kind: 'app',
    name: 'dummy-434510897',
    uri: 'http://su.vg/gotu',
    publicAddr: 'http://viste.cc/eli',
    launchUrl: 'http://gid.me/bubamih',
    awsRoles: [],
    description: 'This is dummy-1681746825 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '18' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://cuzevutuj.by/icbub',
    userGroups: [],
  },
];

const appsWithUserGroups: App[] = [
  {
    id: 'dummy-3834740555',
    kind: 'app',
    name: 'dummy-746715940',
    uri: 'http://kizuud.im/het',
    publicAddr: 'http://div.az/busihuj',
    launchUrl: 'http://ramufica.sk/teba',
    awsRoles: [],
    description: 'This is dummy-2899140136 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '1' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://olfuptad.zw/zecco',
    userGroups: [{ name: 'ug1', description: 'ug1-description' }],
  },
  {
    id: 'dummy-735596623',
    kind: 'app',
    name: 'dummy-3881558958',
    uri: 'http://ima.sd/ivdijwah',
    publicAddr: 'http://kav.cl/zi',
    launchUrl: 'http://woijo.to/anogucfac',
    awsRoles: [],
    description: 'This is dummy-396777662 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '2' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://tefe.sg/kis',
    userGroups: [{ name: 'ug1', description: 'ug1-description' }],
  },
  {
    id: 'dummy-61474169',
    kind: 'app',
    name: 'dummy-2813408209',
    uri: 'http://borrepci.mk/cunnenoc',
    publicAddr: 'http://luwop.km/pagmot',
    launchUrl: 'http://emim.cv/nasdeasa',
    awsRoles: [],
    description: 'This is dummy-1292837400 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '3' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://sudlinot.hn/jomevo',
    userGroups: [],
  },
  {
    id: 'dummy-3147977584',
    kind: 'app',
    name: 'dummy-434510897',
    uri: 'http://su.vg/gotu',
    publicAddr: 'http://viste.cc/eli',
    launchUrl: 'http://gid.me/bubamih',
    awsRoles: [],
    description: 'This is dummy-1681746825 app',
    awsConsole: false,
    samlApp: false,
    labels: [
      { name: 'number', value: '18' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'http://cuzevutuj.by/icbub',
    userGroups: [{ name: 'ug2', description: 'ug2-description' }],
  },
];
