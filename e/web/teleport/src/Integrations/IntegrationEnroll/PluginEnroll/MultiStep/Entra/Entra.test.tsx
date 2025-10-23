import { MemoryRouter, Route } from 'react-router';

import {
  fireEvent,
  render,
  screen,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { pluginsService } from 'e-teleport/services/plugins';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';
import userService from 'teleport/services/user';
import { userEventService } from 'teleport/services/userEvent';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { PluginEnroll } from '../../PluginEnroll';
import { emptyFilter, filterCollection } from './FormMixin';
import { Filters } from './types';

jest.mock('shared/libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

const defaultEnterpriseFlag = cfg.isEnterprise;
const defaultPolicyFlag = cfg.isPolicyEnabled;
const defaultPolicyEntitlement = cfg.entitlements.Policy;
const defaultIdentityEntitlement = cfg.entitlements.Identity;

const authConnectorValue = 'entra-id-custom';
const tenantIdValue = 'some-tenant-id';
const clientIdValue = 'some-client-id';
const tagCachePayload = '<payload>';

let mockedCreatePlugin;
let mockedValidatePlugin;

beforeEach(() => {
  jest
    .spyOn(userEventService, 'captureIntegrationEnrollEvent')
    .mockImplementation();

  mockedCreatePlugin = jest
    .spyOn(pluginsService, 'createStaticAuthPlugin')
    .mockResolvedValue({
      resourceType: 'plugin',
      name: 'entra-id',
      details: 'some-detail',
      statusCode: IntegrationStatusCode.Running,
      kind: 'entra-id',
      spec: {},
    });

  mockedValidatePlugin = jest
    .spyOn(pluginsService, 'validatePlugin')
    .mockResolvedValue(null);

  jest
    .spyOn(pluginsService, 'checkPluginRequiresCleanup')
    .mockResolvedValue(false);

  jest
    .spyOn(userService, 'fetchUsers')
    .mockResolvedValue([{ name: 'alice', roles: [] }]);
});

afterEach(() => {
  jest.clearAllMocks();
  cfg.isEnterprise = defaultEnterpriseFlag;
  cfg.isPolicyEnabled = defaultPolicyFlag;
  cfg.entitlements.Policy = defaultPolicyEntitlement;
  cfg.entitlements.Identity = defaultIdentityEntitlement;
});

test('entra onboard with policy disabled', async () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = false;
  cfg.entitlements.Policy = { enabled: false, limit: 0 };
  cfg.entitlements.Identity = { enabled: true, limit: 0 };

  renderPluginEnroll('entra-id');

  // Test init screen render.
  await expectInitRender(true);
  // TAG support should not be available
  expect(
    screen.getByLabelText('Enable Access Graph integration')
  ).toBeDisabled();

  // Fill initial fields
  setStep1Inputs();

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

  expect(pluginsService.validatePlugin).toHaveBeenCalledTimes(1);
  let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('authConnectorName')).toEqual(
    authConnectorValue
  );
  expect(calledWithFormData.get('defaultOwners')).toEqual('["alice"]');
  expect(calledWithFormData.get('accessGraph')).toBeNull();

  // Finish onboard
  setStep2Inputs();

  await userEvent.click(screen.getByRole('button', { name: /finish/i }));
  expect(screen.getByText(/integrated successfully/i)).toBeInTheDocument();

  expect(pluginsService.createStaticAuthPlugin).toHaveBeenCalledTimes(1);

  calledWithFormData = mockedCreatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('tenantId')).toEqual(tenantIdValue);
  expect(calledWithFormData.get('clientId')).toEqual(clientIdValue);
  expect(calledWithFormData.get('accessGraphCache')).toBeNull();

  expect(
    screen.getByText(/microsoft entra id is integrated successfully/i)
  ).toBeInTheDocument();
});

test('entra onboard with policy enabled', async () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = true;
  cfg.entitlements.Policy = { enabled: true, limit: 0 };
  cfg.entitlements.Identity = { enabled: true, limit: 0 };

  renderPluginEnroll('entra-id');

  // Test init screen render.
  await expectInitRender(false);
  // TAG support should be available and enabled by default
  expect(
    screen.getByLabelText('Enable Access Graph integration')
  ).toBeEnabled();

  // Fill initial fields
  setStep1Inputs();

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

  let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('authConnectorName')).toEqual(
    authConnectorValue
  );
  expect(calledWithFormData.get('defaultOwners')).toEqual('["alice"]');
  expect(calledWithFormData.get('accessGraph')).toEqual('on');

  // Set step 2 inputs (except for TAG cache file).
  // Expect form submission to fail, because the mandatory file field is not set.
  setStep2Inputs();
  await userEvent.click(screen.getByRole('button', { name: /finish/i }));
  expect(
    screen.queryByText(/integrated successfully/i)
  ).not.toBeInTheDocument();
  expect(pluginsService.createStaticAuthPlugin).toHaveBeenCalledTimes(0);

  // Set file upload
  const fileInput = screen.getByTestId('button-file-upload');
  fireEvent.change(fileInput, { target: { files: [createTAGCacheFile()] } });

  // Submit form successfully
  await userEvent.click(screen.getByRole('button', { name: /finish/i }));
  expect(screen.getByText(/integrated successfully/i)).toBeInTheDocument();
  expect(pluginsService.createStaticAuthPlugin).toHaveBeenCalledTimes(1);

  calledWithFormData = mockedCreatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('tenantId')).toEqual(tenantIdValue);
  expect(calledWithFormData.get('clientId')).toEqual(clientIdValue);

  const file = calledWithFormData.get('accessGraphCache');
  expect(file).toBeInstanceOf(File);
  expect(await readFile(file)).toEqual(tagCachePayload);

  expect(
    screen.getByText(/microsoft entra id is integrated successfully/i)
  ).toBeInTheDocument();
});

test('group filter toggle default on', async () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = true;
  cfg.entitlements.Policy = { enabled: true, limit: 0 };
  cfg.entitlements.Identity = { enabled: true, limit: 0 };

  renderPluginEnroll('entra-id');

  // Test init screen render.
  await expectInitRender(false /*access graph locked*/);
  // TAG support should be available and enabled by default
  expect(
    screen.getByLabelText('Enable Access Graph integration')
  ).toBeEnabled();

  expect(screen.getByText('Import All Groups')).toBeInTheDocument();

  // Fill initial fields
  setStep1Inputs();

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

  let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('authConnectorName')).toEqual(
    authConnectorValue
  );
  expect(calledWithFormData.get('defaultOwners')).toEqual('["alice"]');
  expect(calledWithFormData.get('accessGraph')).toEqual('on');
  expect(calledWithFormData.get('groupFilters')).toEqual(
    JSON.stringify(emptyFilter)
  );
});

test('group filter toggle off and configured filters', async () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = true;
  cfg.entitlements.Policy = { enabled: true, limit: 0 };
  cfg.entitlements.Identity = { enabled: true, limit: 0 };

  renderPluginEnroll('entra-id');

  // Test init screen render.
  await expectInitRender(false /*access graph locked*/);
  // TAG support should be available and enabled by default
  expect(
    screen.getByLabelText('Enable Access Graph integration')
  ).toBeEnabled();

  // Group filters
  const importAll = screen.getByText('Import All Groups');
  expect(importAll).toBeInTheDocument();
  // Toggle off
  fireEvent.click(importAll);

  const filter: Filters = {
    id: ['g1', 'g2'],
    nameRegex: ['admin-*'],
    excludeId: ['g2'],
    excludeNameRegex: ['hr*'],
  };
  setFilterInputs(filter);

  // Fill initial fields
  setStep1Inputs();

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

  let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('authConnectorName')).toEqual(
    authConnectorValue
  );
  expect(calledWithFormData.get('defaultOwners')).toEqual('["alice"]');
  expect(calledWithFormData.get('accessGraph')).toEqual('on');
  expect(calledWithFormData.get('groupFilters')).toEqual(
    JSON.stringify(filter)
  );
});

test('group filter toggle on should wipe configured filters', async () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = true;
  cfg.entitlements.Policy = { enabled: true, limit: 0 };
  cfg.entitlements.Identity = { enabled: true, limit: 0 };

  renderPluginEnroll('entra-id');

  // Test init screen render.
  await expectInitRender(false /*access graph locked*/);
  // TAG support should be available and enabled by default
  expect(
    screen.getByLabelText('Enable Access Graph integration')
  ).toBeEnabled();

  const importAll = screen.getByText('Import All Groups');
  expect(importAll).toBeInTheDocument();
  // Toggle off
  fireEvent.click(importAll);

  const filter: Filters = {
    id: ['g1', 'g2'],
    nameRegex: ['admin-*'],
    excludeId: ['g2'],
    excludeNameRegex: ['hr*'],
  };
  setFilterInputs(filter);

  // Toggle on, should wipe out filters
  fireEvent.click(importAll);

  // Fill initial fields
  setStep1Inputs();

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

  let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
  expect(calledWithFormData.get('authConnectorName')).toEqual(
    authConnectorValue
  );
  expect(calledWithFormData.get('defaultOwners')).toEqual('["alice"]');
  expect(calledWithFormData.get('accessGraph')).toEqual('on');
  expect(calledWithFormData.get('groupFilters')).toEqual(
    JSON.stringify(emptyFilter)
  );
});

async function expectInitRender(accessGraphLocked: boolean) {
  const lock = screen.queryByText(/unlock access graph integration/i);
  accessGraphLocked
    ? expect(lock).toBeInTheDocument()
    : expect(lock).not.toBeInTheDocument();

  expect(
    screen.queryByText(/integrated successfully/i)
  ).not.toBeInTheDocument();

  await waitFor(() => {
    expect(screen.getByText(/type a username/i)).toBeInTheDocument();
  });
}

function readFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = reject;
    reader.readAsText(file);
  });
}

async function setStep1Inputs() {
  // Modify the connector name
  const authConnector = screen
    .getByTestId('auth-connector-name')
    .querySelector('input');
  fireEvent.change(authConnector, { target: { value: authConnectorValue } });

  // Select the first user from dropdown.
  const users = screen.getByText(/type a username/i);
  fireEvent.keyDown(users, { key: 'ArrowDown' });
  fireEvent.keyDown(users, { key: 'Enter' });
}

function setStep2Inputs() {
  // Expect post-script fields
  const tenantID = screen.getByPlaceholderText('Tenant ID');
  fireEvent.change(tenantID, { target: { value: tenantIdValue } });
  const clientID = screen.getByPlaceholderText('Client ID');
  fireEvent.change(clientID, { target: { value: clientIdValue } });
}

function createTAGCacheFile() {
  const blob = new Blob([tagCachePayload]);
  return new File([blob], 'cache.json', { type: 'application/json' });
}

function renderPluginEnroll(pluginType: PluginKind, search?: string) {
  const ctx = createTeleportContext();

  render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.getIntegrationEnrollRoute(pluginType), search },
      ]}
    >
      <TeleportContextProvider ctx={ctx}>
        <Route path={cfg.routes.integrationEnroll}>
          <PluginEnroll />
        </Route>
      </TeleportContextProvider>
    </MemoryRouter>
  );
}

function setFilterInputs(filter: Filters) {
  const includeId = screen.getByLabelText(filterCollection[0].label);
  expect(includeId).toBeInTheDocument();
  fireEvent.change(includeId, { target: { value: filter.id[0] } });
  fireEvent.keyDown(includeId, { key: 'Enter' });

  fireEvent.change(includeId, { target: { value: filter.id[1] } });
  fireEvent.keyDown(includeId, { key: 'Enter' });

  const includeNameRegex = screen.getByLabelText(filterCollection[1].label);
  expect(includeNameRegex).toBeInTheDocument();
  fireEvent.change(includeNameRegex, { target: { value: filter.nameRegex } });
  fireEvent.keyDown(includeNameRegex, { key: 'Enter' });

  const excludeId = screen.getByLabelText(filterCollection[2].label);
  expect(excludeId).toBeInTheDocument();
  fireEvent.change(excludeId, { target: { value: filter.excludeId } });
  fireEvent.keyDown(excludeId, { key: 'Enter' });

  const excludeNameRegex = screen.getByLabelText(filterCollection[3].label);
  expect(excludeNameRegex).toBeInTheDocument();
  fireEvent.change(excludeNameRegex, {
    target: { value: filter.excludeNameRegex },
  });
  fireEvent.keyDown(excludeNameRegex, { key: 'Enter' });
}
