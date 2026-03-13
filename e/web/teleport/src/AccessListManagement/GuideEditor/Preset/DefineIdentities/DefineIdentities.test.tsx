import { act, within } from '@testing-library/react';
import { UserEvent } from '@testing-library/user-event';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import selectEvent from 'react-select-event';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { CreateAccessList } from 'e-teleport/AccessListManagement/CreateAccessList/CreateAccessList';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
import cfg from 'teleport/config';
import ResourceService from 'teleport/services/resources';

import { DefinableResourceAccessFields } from '../role/listaccess';
import { AppIdentities } from '../role/resources/app';
import {
  appsWithAllMatchingPermissionSet,
  fetchUnifiedResources,
  makeHandlers,
} from '../TestHelper/mocks';
import { ProviderWithQuery } from '../TestHelper/ProviderWithQuery';

const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListentitlement = cfg.entitlements.AccessLists;

const mio = mockIntersectionObserver();

enableMswServer();

let spiedUnifiedResource;
beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
  spiedUnifiedResource = jest.spyOn(
    ResourceService.prototype,
    'fetchUnifiedResources'
  );

  cfg.isEnterprise = true;
  cfg.entitlements.AccessLists = { enabled: true, limit: 0 };
});

afterEach(async () => {
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
  cfg.isEnterprise = defaultIsEnterpriseFlag;
  cfg.entitlements.AccessLists = defaultAccessListentitlement;
});

test('defining server access renders only server related identity tab', async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);
  await goToAccessTab('node_labels', user);
  await clickLabelAndWaitForRender('NodeTestRow', user);

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/server identities/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();

  expect(
    screen.getByRole('tab', { name: /go to server tab/i })
  ).toBeInTheDocument();

  // Only server tab.
  expect(screen.getAllByRole('tab')).toHaveLength(1);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);
});

test('defining windows access renders only windows related identity tab', async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);
  await goToAccessTab('windows_desktop_labels', user);
  await clickLabelAndWaitForRender('WindowsTestRow', user);

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/windows desktop identities/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();

  expect(
    screen.getByRole('tab', { name: /windows desktop tab/i })
  ).toBeInTheDocument();

  expect(screen.getAllByRole('tab')).toHaveLength(1);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);
});

test('defining db access renders only db related identity tab and test default wildcard values', async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);
  await goToAccessTab('db_labels', user);
  await clickLabelAndWaitForRender('DbTestRow', user);

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/database identities/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();

  expect(
    screen.getByRole('tab', { name: /database tab/i })
  ).toBeInTheDocument();

  expect(screen.getAllByRole('tab')).toHaveLength(1);

  // Verify unsupported input fields are not rendered.
  expect(screen.queryAllByRole('combobox')).toHaveLength(2);
  expect(screen.getByText(/database names/i)).toBeInTheDocument();
  expect(screen.getByText(/database users/i)).toBeInTheDocument();
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);

  // Test default values.
  expect(screen.getAllByText('*')).toHaveLength(2);

  // Delete one wildcard
  await user.click(
    screen.getAllByRole('button', {
      name: 'Remove *',
    })[0]
  );
  expect(screen.getAllByText('*')).toHaveLength(1);

  // Go back to previous step
  await user.click(screen.getByRole('button', { name: /back/i }));
  act(mio.enterAll);
  await screen.findByText('AppTestRow'); // b/c it defaults to app tab

  // Go forward again, should preserve empty state (only 1 wildcard remaining)
  await user.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/database identities/i);
  expect(screen.getAllByText('*')).toHaveLength(1);

  // Go back to delete db access
  await user.click(screen.getByRole('button', { name: /back/i }));
  act(mio.enterAll);
  await screen.findByText('AppTestRow');
  await goToAccessTab('db_labels', user, 'DbTestRow');

  const inputWrapper = screen.getByTestId('resource-label-input');
  await user.click(
    within(inputWrapper).getByRole('button', {
      name: 'Remove env: test',
    })
  );
  act(mio.enterAll);
  await screen.findByText('DbTestRow');
  expect(screen.getByText(/no access defined/i)).toBeInTheDocument();

  // Add db access back
  await clickLabelAndWaitForRender('DbTestRow', user);

  // Go to identities step - should have reset the wildcard values
  await user.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/database identities/i);
  expect(screen.getAllByText('*')).toHaveLength(2);
});

test('defining kube access renders only kube related identity tab', async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);
  await goToAccessTab('kubernetes_labels', user);
  await clickLabelAndWaitForRender('KubeClusterTestRow', user);

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/kubernetes identities/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();

  expect(
    screen.getByRole('tab', { name: /kubernetes cluster tab/i })
  ).toBeInTheDocument();

  expect(screen.getAllByRole('tab')).toHaveLength(1);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);
  expect(
    screen.getByRole('button', { name: /add a kubernetes resource/i })
  ).toBeInTheDocument();
});

test('defining git access does not render any identity tabs (no requirement)', async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);
  await goToAccessTab('github_permissions', user);

  const reactSelectInput = screen.getByRole('combobox');
  await selectEvent.select(reactSelectInput, 'GitServerTestRow');

  expect(screen.getByText('GitServerTestRow')).toBeInTheDocument();
  spiedUnifiedResource.mockClear();

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/no identities are required/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();

  expect(screen.queryAllByRole('tab')).toHaveLength(0);
});

test('defining AWS IC access does not render any identity tabs (no requirement)', async () => {
  const user = userEvent.setup();
  server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);
  spiedUnifiedResource.mockClear();
  await goToAccessTab('awsIc', user);

  // Make a selection.
  await user.click(
    screen.getByRole('button', { name: /make a new selection/i })
  );
  await user.click(
    screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
  );
  await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
  await user.click(screen.getByRole('button', { name: /add selection/i }));
  expect(screen.getAllByTestId(/row-*/i)).toHaveLength(1);

  // Go to identities step.
  // There are two next buttons one from paginated table.
  await user.click(screen.getAllByRole('button', { name: /next/i })[1]);
  await screen.findByText(/no identities are required/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();

  expect(screen.queryAllByRole('tab')).toHaveLength(0);
});

test('defining generic app access does not render identity tabs', async () => {
  const user = userEvent.setup();
  server.use(
    fetchUnifiedResources('get', [
      {
        kind: 'app',
        name: 'AppTestRow',
        labels: [{ name: 'env', value: 'test' }],
      },
    ])
  );

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  // Defaults to application tab.
  await startGuide(user);

  // Selecting a generic app does not require identities
  await clickLabelAndWaitForRender('AppTestRow', user);

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/no identities are required/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
  expect(screen.queryAllByRole('tab')).toHaveLength(0);
});

test('defining application access where identities are required', async () => {
  const testedFields: Record<keyof AppIdentities, boolean> = {
    mcp: false,
    aws_role_arns: false,
    azure_identities: false,
    gcp_service_accounts: false,
  };

  const user = userEvent.setup();
  server.use(
    fetchUnifiedResources('get', [
      {
        kind: 'app',
        name: 'AppTestGcp',
        uri: 'cloud://GCP',
        labels: [{ name: 'env', value: 'test' }],
      },
    ])
  );

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  // Defaults to application tab.
  await startGuide(user);

  // Select any applications, the actual label doesn't matter for this test.
  await clickLabelAndWaitForRender('AppTestGcp', user);

  // Go to identities step.
  await user.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/application identities/i);

  // Test gcp identity renders.
  testedFields['gcp_service_accounts'] = true;
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
  expect(screen.queryAllByRole('tab')).toHaveLength(1);
  expect(screen.getByText(/gcp/i)).toBeInTheDocument();
  expect(screen.queryAllByRole('textbox')).toHaveLength(1);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);

  // Go back to trigger new fetch to include Azure app
  server.use(
    fetchUnifiedResources('get', [
      {
        kind: 'app',
        name: 'AppTestGcp',
        uri: 'cloud://GCP',
      },
      {
        kind: 'app',
        name: 'AppTestAzure',
        uri: 'cloud://Azure',
      },
    ])
  );
  await user.click(screen.getByRole('button', { name: /back/i }));
  act(mio.enterAll);
  await screen.findByText('AppTestAzure');
  spiedUnifiedResource.mockClear();
  await user.click(screen.getByRole('button', { name: /next/i }));

  // Test azure identity is included.
  testedFields['azure_identities'] = true;
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
  expect(screen.queryAllByRole('tab')).toHaveLength(1);
  expect(screen.getByText(/gcp/i)).toBeInTheDocument();
  expect(screen.getByText(/azure/i)).toBeInTheDocument();
  expect(screen.queryAllByRole('textbox')).toHaveLength(2);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);

  // Go back to trigger new fetch to include mcp app
  server.use(
    fetchUnifiedResources('get', [
      {
        kind: 'app',
        name: 'AppTestGcp',
        uri: 'cloud://GCP',
      },
      {
        kind: 'app',
        name: 'AppTestAzure',
        uri: 'cloud://Azure',
      },
      {
        kind: 'app',
        name: 'AppTestMcp',
        subKind: 'mcp',
      },
    ])
  );
  await user.click(screen.getByRole('button', { name: /back/i }));
  act(mio.enterAll);
  await screen.findByText('AppTestMcp');
  spiedUnifiedResource.mockClear();
  await user.click(screen.getByRole('button', { name: /next/i }));

  // Test mcp identity is included.
  testedFields['mcp'] = true;
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
  expect(screen.queryAllByRole('tab')).toHaveLength(1);
  expect(screen.getByText(/gcp/i)).toBeInTheDocument();
  expect(screen.getByText(/azure/i)).toBeInTheDocument();
  expect(screen.getByText(/mcp/i)).toBeInTheDocument();
  expect(screen.queryAllByRole('textbox')).toHaveLength(3);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);

  // Go back to trigger new fetch to include aws consol app
  server.use(
    fetchUnifiedResources('get', [
      {
        kind: 'app',
        name: 'AppTestGcp',
        uri: 'cloud://GCP',
      },
      {
        kind: 'app',
        name: 'AppTestAzure',
        uri: 'cloud://Azure',
      },
      {
        kind: 'app',
        name: 'AppTestMcp',
        subKind: 'mcp',
      },
      {
        kind: 'app',
        name: 'AppTestAwsConsole',
        awsConsole: true,
      },
    ])
  );
  await user.click(screen.getByRole('button', { name: /back/i }));
  act(mio.enterAll);
  await screen.findByText('AppTestAwsConsole');
  spiedUnifiedResource.mockClear();
  await user.click(screen.getByRole('button', { name: /next/i }));

  // Test aws console identity is included.
  testedFields['aws_role_arns'] = true;
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
  expect(screen.queryAllByRole('tab')).toHaveLength(1);
  expect(screen.getByText(/gcp/i)).toBeInTheDocument();
  expect(screen.getByText(/azure/i)).toBeInTheDocument();
  expect(screen.getByText(/mcp/i)).toBeInTheDocument();
  expect(screen.getByText(/aws/i)).toBeInTheDocument();
  expect(screen.queryAllByRole('textbox')).toHaveLength(4);
  expect(screen.queryAllByText(/labels/i)).toHaveLength(0);

  // Ensure all fields got tested.
  expect(testedFields).toEqual({
    mcp: true,
    aws_role_arns: true,
    azure_identities: true,
    gcp_service_accounts: true,
  });
});

test('defining access to everything renders all the correct identity tabs', async () => {
  const addedAccess: Record<DefinableResourceAccessFields, boolean> = {
    app_labels: false,
    db_labels: false,
    kubernetes_labels: false,
    windows_desktop_labels: false,
    node_labels: false,
    awsIc: false,
    github_permissions: false,
  };

  const user = userEvent.setup();
  server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  await startGuide(user);

  /**
   * Define aws ic access
   */
  server.use(fetchUnifiedResources('get', appsWithAllMatchingPermissionSet));
  await goToAccessTab('awsIc', user);
  await user.click(
    screen.getByRole('button', { name: /make a new selection/i })
  );
  await user.click(
    screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
  );
  await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
  await user.click(screen.getByRole('button', { name: /add selection/i }));
  expect(screen.getAllByTestId(/row-*/i)).toHaveLength(1);
  addedAccess['awsIc'] = true;

  /**
   * Define app access
   */
  server.use(
    fetchUnifiedResources('get', [
      {
        kind: 'app',
        name: 'AppTestAzure',
        uri: 'cloud://Azure',
        labels: [{ name: 'env', value: 'test' }],
      },
    ])
  );
  await goToAccessTab('app_labels', user);
  await clickLabelAndWaitForRender('AppTestAzure', user);
  addedAccess['app_labels'] = true;

  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));

  /**
   * Define db access
   */
  await goToAccessTab('db_labels', user);
  await clickLabelAndWaitForRender('DbTestRow', user);
  addedAccess['db_labels'] = true;

  /**
   * Define desktop access
   */
  await goToAccessTab('windows_desktop_labels', user);
  await clickLabelAndWaitForRender('WindowsTestRow', user);
  addedAccess['windows_desktop_labels'] = true;

  /**
   * Define kube access
   */
  await goToAccessTab('kubernetes_labels', user);
  await clickLabelAndWaitForRender('KubeClusterTestRow', user);
  addedAccess['kubernetes_labels'] = true;

  /**
   * Define server access
   */
  await goToAccessTab('node_labels', user);
  await clickLabelAndWaitForRender('NodeTestRow', user);
  addedAccess['node_labels'] = true;

  /**
   * Define git server access
   */
  await goToAccessTab('github_permissions', user);
  const reactSelectInput = screen.getByRole('combobox');
  await selectEvent.select(reactSelectInput, 'GitServerTestRow');
  expect(screen.getByText('GitServerTestRow')).toBeInTheDocument();
  addedAccess['github_permissions'] = true;

  spiedUnifiedResource.mockClear();
  expect(addedAccess).toEqual({
    app_labels: true,
    db_labels: true,
    kubernetes_labels: true,
    windows_desktop_labels: true,
    node_labels: true,
    awsIc: true,
    github_permissions: true,
  });

  /**
   * Go to identities step and test all expected identity tabs are rendered
   */
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/application identities/i);
  expect(spiedUnifiedResource).not.toHaveBeenCalled();
  expect(screen.getAllByRole('tab')).toHaveLength(5);

  expect(
    screen.getByRole('tab', { name: /application tab/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('tab', { name: /database tab/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('tab', { name: /kubernetes cluster tab/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('tab', { name: /windows desktop tab/i })
  ).toBeInTheDocument();
  expect(screen.getByRole('tab', { name: /server tab/i })).toBeInTheDocument();

  /**
   * Test going to different tabs
   */
  await user.click(screen.getByRole('button', { name: /next: database/i }));
  expect(screen.getByText(/database identities/i)).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /next: desktops/i }));
  expect(screen.getByText(/windows desktop identities/i)).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /next: kubernetes/i }));
  expect(screen.getByText(/kubernetes identities/i)).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /next: server/i }));
  expect(screen.getByText(/server identities/i)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /next/i })).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: /next:/i })
  ).not.toBeInTheDocument();

  /**
   * Test clicking back button takes you to previous tab
   */
  await user.click(screen.getByRole('button', { name: /back/i }));
  expect(screen.getByText(/kubernetes identities/i)).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /back/i }));
  expect(screen.getByText(/windows desktop identities/i)).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /back/i }));
  expect(screen.getByText(/database identities/i)).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /back/i }));
  expect(screen.getByText(/application identities/i)).toBeInTheDocument();

  /**
   * Go back to previous step and remove some resource access definitions
   */
  await user.click(screen.getByRole('button', { name: /back/i }));
  act(mio.enterAll);
  await screen.findByText('AppTestRow');

  // Remove app access
  let inputWrapper = screen.getByTestId('resource-label-input');
  await user.click(
    within(inputWrapper).getByRole('button', {
      name: 'Remove env: test',
    })
  );
  act(mio.enterAll);
  await screen.findByText('AppTestRow');

  // Remove db access
  await goToAccessTab('db_labels', user, 'DbTestRow');
  inputWrapper = screen.getByTestId('resource-label-input');
  await user.click(
    within(inputWrapper).getByRole('button', {
      name: 'Remove env: test',
    })
  );
  act(mio.enterAll);
  await screen.findByText('DbTestRow');

  // Remove windows
  await goToAccessTab('windows_desktop_labels', user, 'WindowsTestRow');
  inputWrapper = screen.getByTestId('resource-label-input');
  await user.click(
    within(inputWrapper).getByRole('button', {
      name: 'Remove env: test',
    })
  );
  act(mio.enterAll);
  await screen.findByText('WindowsTestRow');

  /**
   * Go to identities tab again and test identity tabs are as expected
   */
  await user.click(screen.getByRole('button', { name: /next/i }));

  await screen.findByText(/kubernetes identities/i);
  expect(screen.getAllByRole('tab')).toHaveLength(2);

  expect(
    screen.getByRole('tab', { name: /kubernetes cluster tab/i })
  ).toBeInTheDocument();
  expect(screen.getByRole('tab', { name: /server tab/i })).toBeInTheDocument();
}, 15000);

async function startGuide(user: UserEvent) {
  // Select any guide.
  await screen.findByText(/Select the type of Access List/i);
  await user.click(screen.getByText(/temporary access/i));

  await screen.findByText(/define application access/i);
  act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver
  await screen.findByText(/no access defined/i);
}

async function clickLabelAndWaitForRender(id: string, user: UserEvent) {
  let targetRow = screen.getByTestId(id);
  await user.click(within(targetRow).getByTitle(/env: test/i));
  act(mio.enterAll);
  await screen.findByText(id);
  spiedUnifiedResource.mockClear();
}

async function goToAccessTab(
  tabId: DefinableResourceAccessFields,
  user: UserEvent,
  resourceName = ''
) {
  await user.click(screen.getByTestId(tabId));
  expect(await screen.findByText(/define .* access/i)).toBeInTheDocument();
  act(mio.enterAll);
  /* oxlint-disable jest/no-conditional-expect */
  if (resourceName) {
    expect(await screen.findByText(resourceName)).toBeInTheDocument();
  } else {
    expect(await screen.findByText(/no access/i)).toBeInTheDocument();
  }
  /* oxlint-enable jest/no-conditional-expect */
}
