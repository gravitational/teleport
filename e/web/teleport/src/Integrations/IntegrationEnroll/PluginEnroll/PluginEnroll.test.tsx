import React from 'react';
import { MemoryRouter, Route } from 'react-router';
import { render, screen, userEvent } from 'design/utils/testing';
import cfg from 'teleport/config';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import { PluginKind } from 'teleport/services/integrations';

import { PluginEnroll } from './PluginEnroll';

describe('slack PluginEnroll.tsx', () => {
  beforeEach(() => {
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();
  });

  afterEach(() => {
    jest.clearAllMocks();
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
});

function renderPluginEnroll(pluginType: PluginKind, search?: string) {
  render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.getIntegrationEnrollRoute(pluginType), search },
      ]}
    >
      <Route path={cfg.routes.integrationEnroll}>
        <PluginEnroll />
      </Route>
    </MemoryRouter>
  );
}
