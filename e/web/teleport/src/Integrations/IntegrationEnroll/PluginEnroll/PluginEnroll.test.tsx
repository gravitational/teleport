import { http, HttpResponse, PathParams } from 'msw';
import { MemoryRouter, Route, Routes } from 'react-router';

import {
  enableMswServer,
  fireEvent,
  render,
  screen,
  server,
  userEvent,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { APP_GROUP_SYNC_CONFIG } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  pluginsService,
  PluginUpdateRequest,
} from 'e-teleport/services/plugins';
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

enableMswServer();

const defaultIdentityEntitlement = cfg.oss.entitlements.Identity;

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
    await renderPluginEnroll('slack');

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
    await renderPluginEnroll('slack', {
      search: `event_id=${eventId}&success=%7B%22name%22%3A%22slack-default%22%2C%22slack%22%3A%7B%22fallback_channel%22%3A%22%23general-channel%22%7D%7D`,
    });

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

  test('create', async () => {
    jest.spyOn(pluginsService, 'redirectForPluginOAuth').mockResolvedValue();

    await renderPluginEnroll('slack');

    await userEvent.type(
      screen.getByPlaceholderText(/access-requests/i),
      'some-channel'
    );

    await userEvent.click(
      screen.getByRole('button', { name: /connect slack/i })
    );

    expect(pluginsService.redirectForPluginOAuth).toHaveBeenCalledTimes(1);
  });
});

describe('okta PluginEnroll.tsx', () => {
  const stubPlugin = {
    resourceType: 'plugin',
    name: 'okta',
    details: 'some-detail',
    statusCode: IntegrationStatusCode.Running,
    kind: 'okta',
    spec: {},
  };

  beforeEach(() => {
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();

    jest
      .spyOn(userService, 'fetchUsers')
      .mockResolvedValue([{ name: 'apple', roles: [] }]);

    server.use(
      http.get('/v1/enterprise/plugin/okta', () => HttpResponse.json({})),
      http.post('/v1/enterprise/plugins/staticauth', () =>
        HttpResponse.json({})
      ),
      http.post('/v1/enterprise/plugin', () => HttpResponse.json(stubPlugin)),
      http.put('/v1/enterprise/plugin', () => HttpResponse.json(stubPlugin)),
      http.post('/v1/enterprise/pluginconfig/okta/apps', () =>
        HttpResponse.json([{ name: 'Airbase' }])
      ),
      http.post('/v1/enterprise/pluginconfig/okta/groups', () =>
        HttpResponse.json([{ name: 'group-1', description: 'group 1 desc' }])
      )
    );
  });

  afterEach(() => {
    jest.clearAllMocks();
    cfg.oss.entitlements.Identity = defaultIdentityEntitlement;
  });

  test('okta flow without Identity, only SSO step is allowed', async () => {
    server.use(
      http.get('/v1/enterprise/plugins/needscleanup/okta', () =>
        HttpResponse.json({ needsCleanup: false })
      )
    );

    const { ctx } = await renderPluginEnroll('okta', { identity: false });

    expect(
      screen.getByText(/unlock the full integration/i)
    ).toBeInTheDocument();

    await completeFirstSteps({
      ctx,
      metadataUrl:
        'https://some-org-url.okta.com/app/abcdefg/sso/saml/metadata',
    });

    expect(screen.getByText(/sso connected!/i)).toBeInTheDocument();
    // Shouldn't show the next step if not entitled to Identity.
    expect(screen.queryByText(/next – scim/i)).not.toBeInTheDocument();
  });

  test('okta flow with Identity, without custom filters', async () => {
    server.use(
      http.get('/v1/enterprise/plugins/needscleanup/okta', () =>
        HttpResponse.json({ needsCleanup: false })
      )
    );

    const { ctx } = await renderPluginEnroll('okta', { identity: true });

    expect(
      screen.queryByText(/unlock the full integration/i)
    ).not.toBeInTheDocument();

    await completeFirstSteps({
      ctx,
      metadataUrl:
        'https://some-org-url.okta.com/app/abcdefg/sso/saml/metadata',
      scim: true,
      clientID: 'some-client-id',
    });

    expect(screen.getByText(/sync all user groups/i)).toBeInTheDocument();

    // Test all the okta tables rendered.
    expect(screen.getByText(/airbase/i)).toBeInTheDocument();
    expect(screen.getByText(/group-1/i)).toBeInTheDocument();

    // Select the first user from dropdown.
    const users = screen.getByText(/type a username/i);
    fireEvent.keyDown(users, { key: 'ArrowDown' });
    fireEvent.keyDown(users, { key: 'Enter' });

    await userEvent.click(screen.getByRole('button', { name: /continue/i }));
    expect(
      screen.getByText(APP_GROUP_SYNC_CONFIG.completeCopy.title)
    ).toBeInTheDocument();
  });

  test('okta flow with Identity, with custom filters', async () => {
    server.use(
      http.get('/v1/enterprise/plugins/needscleanup/okta', () =>
        HttpResponse.json({ needsCleanup: false })
      )
    );

    const { ctx } = await renderPluginEnroll('okta', { identity: true });

    expect(
      screen.queryByText(/unlock the full integration/i)
    ).not.toBeInTheDocument();

    await completeFirstSteps({
      ctx,
      metadataUrl:
        'https://some-org-url.okta.com/app/abcdefg/sso/saml/metadata',
      scim: true,
      clientID: 'some-client-id',
    });

    // Test user/group screen is rendered.
    expect(
      screen.getByText(/Sync User Groups and App Assignments/i)
    ).toBeInTheDocument();

    // Wait for okta tables to render.
    await screen.findByText(/airbase/i);
    await screen.findByText(/group-1/i);

    // Select the first user from dropdown.
    const users = screen.getByText(/type a username/i);
    fireEvent.keyDown(users, { key: 'ArrowDown' });
    fireEvent.keyDown(users, { key: 'Enter' });

    // Define a group filter.
    fireEvent.click(
      screen.getByText(/sync all user groups/i).previousElementSibling
    );
    const groupFilter = screen.getByLabelText(/filter by group name/i);
    fireEvent.change(groupFilter, { target: { value: '^group*' } });
    fireEvent.keyDown(groupFilter, { key: 'Enter' });

    // Define a app filter.
    fireEvent.click(screen.getByText(/sync all apps/i).previousElementSibling);
    const appFilter = screen.getByLabelText(/filter by app name/i);
    fireEvent.change(appFilter, { target: { value: 'app-*' } });
    fireEvent.keyDown(appFilter, { key: 'Enter' });

    server.use(
      http.put<PathParams, PluginUpdateRequest<'okta'>>(
        '/v1/enterprise/plugin',
        async ({ request }) => {
          const body = await request.json();

          if (!body.okta[FormDataField.GroupFilters].includes('^group*')) {
            return HttpResponse.json(
              { error: { message: 'invalid group filter' } },
              { status: 500 }
            );
          }

          if (!body.okta[FormDataField.AppFilters].includes('app-*')) {
            return HttpResponse.json(
              { error: { message: 'invalid app filter' } },
              { status: 500 }
            );
          }

          return HttpResponse.json(stubPlugin);
        }
      )
    );

    // Submit and test the submitted data.
    await userEvent.click(screen.getByRole('button', { name: /continue/i }));

    expect(screen.queryByText(/invalid app filter/)).not.toBeInTheDocument();
    expect(screen.queryByText(/invalid group filter/)).not.toBeInTheDocument();
  });

  test('okta flow, requiring clean up', async () => {
    server.use(
      http.get('/v1/enterprise/plugins/needscleanup/okta', () =>
        HttpResponse.json({ needsCleanup: true })
      )
    );

    jest.spyOn(pluginsService, 'cleanupPlugin').mockResolvedValue(null);

    await renderPluginEnroll('okta', { identity: true });

    expect(
      screen.queryByText(/unlock the full integration/i)
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByText(/set up single sign-on/i));

    expect(screen.getByText(/cleanup required/i)).toBeInTheDocument();

    // Canceling should re-render the cleanup dialogue.
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByText(/cleanup required/i)).not.toBeInTheDocument();
    await userEvent.click(screen.getByText(/set up single sign-on/i));
    expect(screen.getByText(/cleanup required/i)).toBeInTheDocument();

    // Clicking on clean up should close the dialogue.
    await userEvent.click(screen.getByRole('button', { name: /clean up/i }));
    expect(pluginsService.cleanupPlugin).toHaveBeenCalledTimes(1);

    expect(screen.queryByText('cleanup required')).not.toBeInTheDocument();
  });

  test('okta flow with user sync & scim enabled, custom filter error handling', async () => {
    server.use(
      http.get('/v1/enterprise/plugins/needscleanup/okta', () =>
        HttpResponse.json({ needsCleanup: false })
      )
    );

    const { ctx } = await renderPluginEnroll('okta', { identity: true });

    expect(
      screen.queryByText(/unlock the full integration/i)
    ).not.toBeInTheDocument();

    await completeFirstSteps({
      ctx,
      metadataUrl:
        'https://some-org-url.okta.com/app/abcdefg/sso/saml/metadata',
      scim: true,
      clientID: 'some-client-id',
    });

    await screen.findByText(/group name/i);
    await screen.findByText(/airbase/i);

    server.use(
      http.post('/v1/enterprise/pluginconfig/okta/apps', () =>
        HttpResponse.json(
          { error: { message: 'invalid filter app-' } },
          { status: 400 }
        )
      )
    );

    server.use(
      http.post('/v1/enterprise/pluginconfig/okta/groups', () => {
        return HttpResponse.json(
          { error: { message: 'invalid filter group-' } },
          { status: 400 }
        );
      })
    );
    // Select the first user from dropdown.
    const users = screen.getByText(/type a username/i);
    fireEvent.keyDown(users, { key: 'ArrowDown' });
    fireEvent.keyDown(users, { key: 'Enter' });

    // Define a invalid group filter.
    fireEvent.click(
      screen.getByText(/sync all user groups/i).previousElementSibling
    );
    const groupFilter = screen.getByLabelText(/filter by group name/i);
    fireEvent.change(groupFilter, { target: { value: 'group-' } });
    fireEvent.keyDown(groupFilter, { key: 'Enter' });

    await screen.findByText(/the following filters are invalid: group-/i);

    // Define a invalid app filter.
    fireEvent.click(screen.getByText(/sync all apps/i).previousElementSibling);
    const appFilter = screen.getByLabelText(/filter by app name/i);
    fireEvent.change(appFilter, { target: { value: 'app-' } });
    fireEvent.keyDown(appFilter, { key: 'Enter' });

    await screen.findByText(/the following filters are invalid: app-/i);

    // Invalid states prevent user from going to next step.
    const submitButton = screen.getByRole('button', { name: /continue/i });

    expect(submitButton).toBeDisabled();
  });

  test('okta create error', async () => {
    server.use(
      http.get('/v1/enterprise/plugins/needscleanup/okta', () =>
        HttpResponse.json({ needsCleanup: false })
      )
    );

    jest
      .spyOn(pluginsService, 'createStaticAuthPlugin')
      .mockRejectedValue(new Error('some create error'));

    const { ctx } = await renderPluginEnroll('okta', { identity: false });

    expect(
      screen.getByText(/unlock the full integration/i)
    ).toBeInTheDocument();

    await completeFirstSteps({
      ctx,
      metadataUrl:
        'https://some-org-url.okta.com/app/abcdefg/sso/saml/metadata',
    });

    expect(pluginsService.createStaticAuthPlugin).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/some create error/i)).toBeInTheDocument();
  });
});

const completeFirstSteps = async ({
  ctx,
  metadataUrl,
  scim,
  clientID,
}: {
  ctx: ReturnType<typeof createTeleportContextE>;
  metadataUrl: string;
  scim?: boolean;
  clientID?: string;
}) => {
  jest.spyOn(ctx.resourceService, 'fetchAuthConnectors').mockResolvedValue({
    defaultConnector: undefined,
    connectors: [],
  });

  await userEvent.click(screen.getByText(/set up single sign-on/i));
  await screen.findByText(/configure sso/i);
  fireEvent.change(screen.getByLabelText(/metadata url/i), {
    target: {
      value: metadataUrl,
    },
  });
  await userEvent.click(screen.getByRole('button', { name: /continue/i }));
  if (!scim) return;
  await userEvent.click(screen.getByText(/next/i));
  await userEvent.click(
    screen.getByRole('button', { name: /save scim configuration/i })
  );
  await userEvent.click(screen.getByRole('button', { name: /continue/i }));
  await userEvent.click(screen.getByText(/next/i));
  if (!clientID) return;
  fireEvent.change(screen.getByLabelText(/client id/i), {
    target: {
      value: clientID,
    },
  });
  await userEvent.click(screen.getByText(/continue/i));
  await userEvent.click(screen.getByText(/next/i));
};

async function renderPluginEnroll(
  pluginType: PluginKind,
  {
    search,
    identity = false,
  }: {
    search?: string;
    identity?: boolean;
  } = {}
) {
  const ctx = createTeleportContextE();
  cfg.oss.entitlements.Identity = { enabled: identity, limit: 0 };

  render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.oss.getIntegrationEnrollRoute(pluginType), search },
      ]}
    >
      <TeleportContextProvider ctx={ctx}>
        <Routes>
          <Route
            path={`${cfg.oss.routes.integrationEnroll}/*`}
            element={<PluginEnroll />}
          />
        </Routes>
      </TeleportContextProvider>
    </MemoryRouter>
  );

  if (pluginType === 'okta') {
    await screen.findByText(/okta integration overview/i);
  }

  return { ctx };
}
