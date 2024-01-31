import React from 'react';
import { MemoryRouter, Route } from 'react-router';
import { fireEvent, render, screen, userEvent } from 'design/utils/testing';
import cfg from 'teleport/config';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { pluginsService } from 'e-teleport/services/plugins';

import { PluginEnroll } from './PluginEnroll';

const defaultIgsFlag = cfg.isIgsEnabled;

describe('slack and okta PluginEnroll.tsx', () => {
  let mockedCreatePlugin;
  let mockedValidatePlugin;
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
  });

  afterEach(() => {
    jest.clearAllMocks();
    cfg.isIgsEnabled = defaultIgsFlag;
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

  test('enroll failed state', async () => {
    const eventId = 'c6b794e1-afcf-4e16-ac5b';

    renderPluginEnroll(
      'slack',
      `event_id=${eventId}&error=some-error&error_description=some%20error%20description`
    );

    expect(screen.getByText(/unable to connect Slack/i)).toBeInTheDocument();
    expect(
      userEventService.captureIntegrationEnrollEvent
    ).not.toHaveBeenCalled();

    // Test that the correct error param is extracted correctly.
    expect(screen.getByText(/some error description/i)).toBeInTheDocument();

    // Test closing the dialog, renders the slack page.
    await userEvent.click(screen.getByRole('button', { name: /close/i }));
    expect(
      screen.getByText(/Slack access request notifications/i)
    ).toBeInTheDocument();
  });

  test('okta flow without igs enabled', async () => {
    cfg.isIgsEnabled = false;

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

  test('okta flow with igs enabled', async () => {
    cfg.isIgsEnabled = true;

    renderPluginEnroll('okta');

    // Test init screen render.
    expect(screen.queryByText(/unlock user sync/i)).not.toBeInTheDocument();
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

    // Test plugin validation api call.
    await userEvent.click(
      screen.getByRole('button', { name: /connect okta/i })
    );

    const testFormData = new FormData();
    testFormData.append('orgURL', 'some-org-url.com');
    testFormData.append('apiToken', 'some-token-value');

    expect(pluginsService.validatePlugin).toHaveBeenCalledTimes(1);
    let calledWithFormData = mockedValidatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('orgURL')).toEqual(
      testFormData.get('orgURL')
    );
    expect(calledWithFormData.get('apiToken')).toEqual(
      testFormData.get('apiToken')
    );

    expect(pluginsService.createPlugin).toHaveBeenCalledTimes(1);
    calledWithFormData = mockedCreatePlugin.mock.calls[0][0];
    expect(calledWithFormData.get('orgURL')).toEqual(
      testFormData.get('orgURL')
    );
    expect(calledWithFormData.get('apiToken')).toEqual(
      testFormData.get('apiToken')
    );
    expect(calledWithFormData.get('scimToken')).not.toBeFalsy();

    // Test Scim view rendered.
    expect(screen.getByText(/okta scim/i)).toBeInTheDocument();
    expect(
      screen.getByText(`${cfg.baseUrl}/v1/webapi/scim/okta`)
    ).toBeInTheDocument();

    // Test finish render
    await userEvent.click(screen.getByRole('button', { name: /finish/i }));
    expect(screen.getByText(/integrated successfully/i)).toBeInTheDocument();
  });

  test('okta validation error', async () => {
    jest
      .spyOn(pluginsService, 'validatePlugin')
      .mockRejectedValue(new Error('some validation error'));

    renderPluginEnroll('okta');

    // Enter input field.
    const orgUrlInput = screen.getByPlaceholderText(
      /examplecompanyname.okta.com/i
    );
    fireEvent.change(orgUrlInput, { target: { value: 'some-org-url.com' } });

    const tokenInput = screen.getByPlaceholderText(
      /00QCjAl4MlV-WPXM...0HmjFx-vbGua/i
    );
    fireEvent.change(tokenInput, { target: { value: 'some-token-value' } });

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
    jest
      .spyOn(pluginsService, 'createPlugin')
      .mockRejectedValue(new Error('some create error'));

    renderPluginEnroll('okta');

    // Enter input field.
    const orgUrlInput = screen.getByPlaceholderText(
      /examplecompanyname.okta.com/i
    );
    fireEvent.change(orgUrlInput, { target: { value: 'some-org-url.com' } });

    const tokenInput = screen.getByPlaceholderText(
      /00QCjAl4MlV-WPXM...0HmjFx-vbGua/i
    );
    fireEvent.change(tokenInput, { target: { value: 'some-token-value' } });

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
