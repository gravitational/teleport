import { MemoryRouter, Route } from 'react-router';

import { fireEvent, render, screen, userEvent } from 'design/utils/testing';

import { pluginsService } from 'e-teleport/services/plugins';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';
import userService from 'teleport/services/user';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { PluginEnroll } from './PluginEnroll';

jest.mock('shared/libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

const defaultSyncEntitlement = cfg.entitlements.OktaUserSync;
const defaultScimEntitlement = cfg.entitlements.OktaSCIM;

describe('slack PluginEnroll.tsx', () => {
  beforeEach(() => {
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();
  });

  afterEach(() => {
    jest.clearAllMocks();
    cfg.entitlements.OktaUserSync = defaultSyncEntitlement;
    cfg.entitlements.OktaSCIM = defaultScimEntitlement;
  });

  test('missing input prevents submitting', async () => {
    renderPluginEnroll('slack');

    expect(
      screen.getByText(/Slack access request notifications/i)
    ).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole('button', { name: /connect Slack/i })
    );
    expect(
      screen.getByText(/default channel must be specified/i)
    ).toBeInTheDocument();
  });

  test('enroll success state', async () => {
    const eventId = 'c6b794e1-afcf-4e16-ac5b';
    renderPluginEnroll(
      'slack',
      `event_id=${eventId}&success=%7B%22name%22%3A%22slack-default%22%2C%22slack%22%3A%7B%22fallback_channel%22%3A%22%23general-channel%22%7D%7D`
    );

    // Test that the correct param is used and successful JSON parsing.
    expect(screen.getByText(/#general-channel/i)).toBeInTheDocument();
    expect(
      screen.getByText(/slack is integrated successfully/i)
    ).toBeInTheDocument();

    expect(
      userEventService.captureIntegrationEnrollEvent
    ).toHaveBeenLastCalledWith({
      event: IntegrationEnrollEvent.Complete,
      eventData: {
        id: eventId,
        kind: IntegrationEnrollKind.Slack,
      },
    });
  });
});

describe('okta PluginEnroll.tsx', () => {
  let mockedCreatePlugin;
  let mockedValidatePlugin;
  let mockedGetOktaGroups;
  let mockedGetOktaApps;
  beforeEach(() => {
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();

    mockedCreatePlugin = jest
      .spyOn(pluginsService, 'createPlugin')
      .mockResolvedValue({
        resourceType: 'plugin',
        name: 'okta',
        details: 'some-detail',
        statusCode: IntegrationStatusCode.Running,
        kind: 'okta',
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
      .mockResolvedValue([{ name: 'apple', roles: [] }]);

    mockedGetOktaApps = jest
      .spyOn(pluginsService, 'getPluginConfigOktaApps')
      .mockResolvedValue([{ name: 'Airbase' }]);

    mockedGetOktaGroups = jest
      .spyOn(pluginsService, 'getPluginConfigOktaGroups')
      .mockResolvedValue([{ name: 'group-1', description: 'group 1 desc' }]);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('okta flow without user sync and without scim, only the first step is allowed', async () => {
    cfg.entitlements.OktaUserSync = { enabled: false, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: false, limit: 0 };

    renderPluginEnroll('okta');

    // Test init screen render.
    expect(screen.getByText(/unlock user sync/i)).toBeInTheDocument();
    expect(
      screen.queryByText(/integrated successfully/i)
    ).not.toBeInTheDocument();

    // Test input field.
    const orgUrlInput = screen.getByPlaceholderText(
      /examplecompanyname.okta.com/i
    );
    fireEvent.change(orgUrlInput, { target: { value: 'some-org-url.com' } });

    const tokenInput = screen.getByPlaceholderText(
      /00QCjAl4MlV-WPXM...0HmjFx-vbGua/i
    );
    fireEvent.change(tokenInput, { target: { value: 'some-token-value' } });

    // Test plugin install.
    await userEvent.click(
      screen.getByRole('button', { name: /connect okta/i })
    );

    const testFormData = new FormData();
    testFormData.append('orgURL', 'some-org-url.com');
    testFormData.append('apiToken', 'some-token-value');

    // Test okta validation api call.
    expect(pluginsService.validatePlugin).toHaveBeenCalledTimes(1);
    let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('orgURL')).toEqual(
      testFormData.get('orgURL')
    );
    expect(calledWithFormData.get('apiToken')).toEqual(
      testFormData.get('apiToken')
    );
    expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

    // Test okta install api call.
    expect(pluginsService.createPlugin).toHaveBeenCalledTimes(1);
    calledWithFormData = mockedCreatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('orgURL')).toEqual(
      testFormData.get('orgURL')
    );
    expect(calledWithFormData.get('apiToken')).toEqual(
      testFormData.get('apiToken')
    );
    expect(calledWithFormData.get('scimToken')).toBeFalsy();

    // Test after installation, finish screen is rendered.
    expect(screen.getByText(/integrated successfully/i)).toBeInTheDocument();
  });

  test('okta flow with user sync & scim enabled, default (no custom filters)', async () => {
    cfg.entitlements.OktaUserSync = { enabled: true, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: true, limit: 0 };

    renderPluginEnroll('okta');

    // Test init screen render.
    expect(screen.queryByText(/unlock user sync/i)).not.toBeInTheDocument();
    expect(
      screen.queryByText(/integrated successfully/i)
    ).not.toBeInTheDocument();

    await fillInFirstStepInputs();

    // Test plugin validation api call.
    await userEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

    const testFormData = new FormData();
    testFormData.append('orgURL', 'https://some-org-url.com');
    testFormData.append('apiToken', 'some-token-value');

    expect(pluginsService.validatePlugin).toHaveBeenCalledTimes(1);
    let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('orgURL')).toEqual(
      testFormData.get('orgURL')
    );
    expect(calledWithFormData.get('apiToken')).toEqual(
      testFormData.get('apiToken')
    );

    // Test user group  screen is rendered.
    expect(
      screen.getByText(
        /Configure Syncing of User Groups and Direct Assignments/i
      )
    ).toBeInTheDocument();

    // Test all the okta tables rendered.
    await screen.findByText(/airbase/i);
    expect(screen.getByText(/airbase/i)).toBeInTheDocument();

    await screen.findByText(/group-1/i);
    expect(screen.getByText(/group-1/i)).toBeInTheDocument();

    // Select the first user from dropdown.
    const users = screen.getByText(/type a username/i);
    fireEvent.keyDown(users, { key: 'ArrowDown' });
    fireEvent.keyDown(users, { key: 'Enter' });

    const btns = screen.getAllByRole('button', { name: /next/i });
    await userEvent.click(btns[btns.length - 1]);

    // Test okta install api call.
    expect(pluginsService.createPlugin).toHaveBeenCalledTimes(1);
    calledWithFormData = mockedCreatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('orgURL')).toEqual(
      testFormData.get('orgURL')
    );
    expect(calledWithFormData.get('apiToken')).toEqual(
      testFormData.get('apiToken')
    );
    expect(calledWithFormData.get('scimToken')).not.toBeFalsy();
    expect(calledWithFormData.get('appFilters')).toBeNull();
    expect(calledWithFormData.get('groupFilters')).toBeNull();
    expect(calledWithFormData.get('defaultOwners')).toBe(
      JSON.stringify(['apple'])
    );

    // Test Scim view rendered.
    expect(screen.getByText(/okta scim/i)).toBeInTheDocument();
    expect(
      screen.getByText(`${cfg.baseUrl}/v1/webapi/scim/okta`)
    ).toBeInTheDocument();

    // Test finish render
    await userEvent.click(screen.getByRole('button', { name: /finish/i }));
    expect(screen.getByText(/integrated successfully/i)).toBeInTheDocument();
  });

  test('okta flow with user sync & scim enabled, with app & group custom filters', async () => {
    cfg.entitlements.OktaUserSync = { enabled: true, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: true, limit: 0 };

    renderPluginEnroll('okta');
    await fillInFirstStepInputs();

    // Go to next step.
    await userEvent.click(screen.getByRole('button', { name: /next/i }));

    // Wait for okta tables to render.
    await screen.findByText(/airbase/i);
    await screen.findByText(/group-1/i);

    // Select the first user from dropdown.
    const users = screen.getByText(/type a username/i);
    fireEvent.keyDown(users, { key: 'ArrowDown' });
    fireEvent.keyDown(users, { key: 'Enter' });

    // Define a group filter.
    fireEvent.click(screen.getByText(/import all user groups/i));
    const groupFilter = screen.getByLabelText('input-group');
    fireEvent.change(groupFilter, { target: { value: '^group*' } });
    fireEvent.keyDown(groupFilter, { key: 'Enter' });

    // Define a app filter.
    fireEvent.click(
      screen.getAllByText(/import all apps with direct assignments/i)[0]
    );
    const appFilter = screen.getByLabelText('input-app');
    fireEvent.change(appFilter, { target: { value: 'app-*' } });
    fireEvent.keyDown(appFilter, { key: 'Enter' });

    // Test okta install api call.
    const btns = screen.getAllByRole('button', { name: /next/i });
    await userEvent.click(btns[btns.length - 1]);
    const calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('groupFilters')).toBe(
      JSON.stringify(['^group*'])
    );
    expect(calledWithFormData.get('appFilters')).toBe(
      JSON.stringify(['app-*'])
    );
  });

  test('okta flow with user sync & scim enabled, skipping step', async () => {
    cfg.entitlements.OktaUserSync = { enabled: true, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: true, limit: 0 };

    renderPluginEnroll('okta');
    await fillInFirstStepInputs();

    // Go to next step.
    await userEvent.click(screen.getByRole('button', { name: /next/i }));

    // Wait for okta tables to render.
    await screen.findByText(/airbase/i);
    await screen.findByText(/group-1/i);

    await userEvent.click(screen.getByRole('button', { name: /skip/i }));

    const calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('groupFilters')).toBeNull();
    expect(calledWithFormData.get('appFilters')).toBeNull();
    expect(calledWithFormData.get('defaultOwners')).toBeNull();
  });

  test('okta flow, requiring clean up', async () => {
    cfg.entitlements.OktaUserSync = { enabled: true, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: true, limit: 0 };

    jest
      .spyOn(pluginsService, 'checkPluginRequiresCleanup')
      .mockResolvedValue(true);

    jest.spyOn(pluginsService, 'cleanupPlugin').mockResolvedValue(null);

    renderPluginEnroll('okta');
    await fillInFirstStepInputs();

    // Go to next step.
    await userEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(pluginsService.checkPluginRequiresCleanup).toHaveBeenCalledTimes(1);

    expect(screen.getByText(/cleanup required/i)).toBeInTheDocument();

    // Canceling should re-render the clean up dialogue.
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByText(/cleanup required/i)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(screen.getByText(/cleanup required/i)).toBeInTheDocument();

    // Clicking on clean up should close the dialogue.
    await userEvent.click(screen.getByRole('button', { name: /clean up/i }));
    expect(pluginsService.cleanupPlugin).toHaveBeenCalledTimes(1);

    expect(screen.queryByText('cleanup required')).not.toBeInTheDocument();
  });

  test('okta flow with user sync & scim enabled, custom filter error handling', async () => {
    cfg.entitlements.OktaUserSync = { enabled: true, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: true, limit: 0 };

    renderPluginEnroll('okta');
    await fillInFirstStepInputs();

    // Go to next step.
    await userEvent.click(screen.getByRole('button', { name: /next/i }));

    await screen.findByText(/group name/i);
    await screen.findByText(/airbase/i);

    jest.resetAllMocks();
    mockedGetOktaApps = jest
      .spyOn(pluginsService, 'getPluginConfigOktaApps')
      .mockRejectedValue(
        new ApiError('invalid filter app-', { status: 400 } as Response)
      );
    mockedGetOktaGroups = jest
      .spyOn(pluginsService, 'getPluginConfigOktaGroups')
      .mockRejectedValue(
        new ApiError('invalid filter group-', { status: 400 } as Response)
      );

    // Select the first user from dropdown.
    const users = screen.getByText(/type a username/i);
    fireEvent.keyDown(users, { key: 'ArrowDown' });
    fireEvent.keyDown(users, { key: 'Enter' });

    // Define a invalid group filter.
    fireEvent.click(screen.getByText(/import all user groups/i));
    const groupFilter = screen.getByLabelText('input-group');
    fireEvent.change(groupFilter, { target: { value: 'group-' } });
    fireEvent.keyDown(groupFilter, { key: 'Enter' });

    expect(mockedGetOktaGroups).toHaveBeenCalledTimes(1);
    await screen.findByText(/the following filters are invalid: group-/i);

    // Define a invalid app filter.
    fireEvent.click(
      screen.getByText(/import all apps with direct assignments/i)
    );
    const appFilter = screen.getByLabelText('input-app');
    fireEvent.change(appFilter, { target: { value: 'app-' } });
    fireEvent.keyDown(appFilter, { key: 'Enter' });

    expect(mockedGetOktaApps).toHaveBeenCalledTimes(1);
    await screen.findByText(/the following filters are invalid: app-/i);

    // Invalid states prevent user from going to next step.
    const btns = screen.getAllByRole('button', { name: /next/i });
    await userEvent.click(btns[btns.length - 1]);
    expect(mockedCreatePlugin).not.toHaveBeenCalled();
  });

  test('okta validation error', async () => {
    cfg.entitlements.OktaUserSync = { enabled: false, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: false, limit: 0 };

    jest
      .spyOn(pluginsService, 'validatePlugin')
      .mockRejectedValue(new Error('some validation error'));

    renderPluginEnroll('okta');
    await fillInFirstStepInputs();

    // Test the api call error.
    await userEvent.click(
      screen.getByRole('button', { name: /connect okta/i })
    );

    expect(pluginsService.validatePlugin).toHaveBeenCalledTimes(1);
    expect(pluginsService.createPlugin).not.toHaveBeenCalled();

    // Test error rendered
    expect(screen.getByText(/some validation error/i)).toBeInTheDocument();
  });

  test('okta create error', async () => {
    cfg.entitlements.OktaUserSync = { enabled: false, limit: 0 };
    cfg.entitlements.OktaSCIM = { enabled: false, limit: 0 };

    jest
      .spyOn(pluginsService, 'createPlugin')
      .mockRejectedValue(new Error('some create error'));

    renderPluginEnroll('okta');
    await fillInFirstStepInputs();

    // Test the api call error.
    await userEvent.click(
      screen.getByRole('button', { name: /connect okta/i })
    );

    expect(pluginsService.validatePlugin).toHaveBeenCalledTimes(1);
    expect(pluginsService.createPlugin).toHaveBeenCalledTimes(1);

    // Test error rendered
    expect(screen.getByText(/some create error/i)).toBeInTheDocument();
  });
});

async function fillInFirstStepInputs() {
  // Enter input fields

  const orgUrlInput = screen.getByPlaceholderText(
    /examplecompanyname.okta.com/i
  );
  fireEvent.change(orgUrlInput, { target: { value: 'some-org-url.com' } });

  const tokenInput = screen.getByPlaceholderText(
    /00QCjAl4MlV-WPXM...0HmjFx-vbGua/i
  );
  fireEvent.change(tokenInput, { target: { value: 'some-token-value' } });
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
