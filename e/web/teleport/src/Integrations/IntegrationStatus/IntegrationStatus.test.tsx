/* eslint-disable testing-library/no-node-access */
import { within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { MemoryRouter, Route, Routes } from 'react-router';

import {
  act,
  enableMswServer,
  fireEvent,
  render,
  screen,
  server,
  tick,
  userEvent,
} from 'design/utils/testing';

import { OktaIntegrationStepType } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { IntegrationStatus } from 'e-teleport/Integrations/IntegrationStatus/IntegrationStatus';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'teleport/config';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

const defaultIdentity = cfg.entitlements.Identity;

enableMswServer();

describe('Okta status', () => {
  afterEach(() => {
    jest.clearAllMocks();
    cfg.entitlements.Identity = defaultIdentity;
  });

  test('does not show unsupported message', async () => {
    server.use(
      http.get('/v1/enterprise/plugin/some-name', () =>
        HttpResponse.json(stubOktaPluginOnlySSO)
      )
    );

    const ctx = createTeleportContextE();

    render(
      <MemoryRouter
        initialEntries={[`/web/integrations/status/okta/some-name`]}
      >
        <TeleportContextProvider ctx={ctx}>
          <Routes>
            <Route
              path={`${cfg.routes.integrationStatus}/*`}
              element={<IntegrationStatus />}
            />
          </Routes>
        </TeleportContextProvider>
      </MemoryRouter>
    );
    await act(tick);

    expect(
      screen.queryByText(`Status for integration type okta is not supported`)
    ).not.toBeInTheDocument();
    expect(screen.getByText('Okta Integration')).toBeInTheDocument();
  });

  test('renders CTA without Identity entitlement', async () => {
    server.use(
      http.get('/v1/enterprise/plugin/okta', () =>
        HttpResponse.json(stubOktaPluginOnlySSO)
      )
    );

    await renderOktaStatus();

    expect(
      await screen.findByText(stubOktaPluginOnlySSO.spec.orgUrl)
    ).toBeInTheDocument();
    expect(
      screen.getByText(stubOktaPluginOnlySSO.spec.oktaAppId)
    ).toBeInTheDocument();
    // CTA should be displayed for SCIM, UserSync, and App/Group Sync.
    expect(
      screen.getAllByText(/unlock with teleport identity governance/i)
    ).toHaveLength(3);
  });

  test('allows enabling SCIM, UserSync, and App/Group Sync after setup', async () => {
    server.use(
      http.get('/v1/enterprise/plugin/okta', () =>
        HttpResponse.json(stubOktaPluginOnlySSO)
      ),
      http.put('/v1/enterprise/plugin', () =>
        HttpResponse.json({
          ...stubOktaPluginOnlySSO,
          spec: {
            ...stubOktaPluginOnlySSO.spec,
            enableUserSync: true,
          },
          status: {
            ...stubOktaPluginOnlySSO.status,
            details: {
              ...stubOktaPluginOnlySSO.status.details,
              okta: {
                ...stubOktaPluginOnlySSO.status.details.okta,
                users_sync_details: {
                  enabled: true,
                  last_successful: new Date(Date.now() - 1000 * 60),
                  last_failed: null,
                  num_users_synced: 20,
                },
              },
            },
          },
        })
      )
    );

    await renderOktaStatus(true);

    expect(
      screen.queryByText(/unlock with teleport identity governance/i)
    ).not.toBeInTheDocument();

    let userSyncSection = (await screen.findByText(/user sync/i)).closest('div')
      ?.parentElement?.parentElement;
    await userEvent.click(
      within(userSyncSection).getByRole('button', { name: /options/i })
    );
    await userEvent.click(screen.getByRole('menuitem', { name: /enable/i }));

    expect(screen.getByText(/sync users/i)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/client id/i), {
      target: { value: 'some-client-id' },
    });
    await userEvent.click(
      screen.getByRole('button', { name: /save changes/i })
    );
    await act(tick);

    userSyncSection = (await screen.findByText(/user sync/i)).closest('div')
      ?.parentElement?.parentElement;
    expect(within(userSyncSection).getByText(/enabled/i)).toBeInTheDocument();
    expect(within(userSyncSection).getByText('20')).toBeInTheDocument();
  });
});

const stubOktaPluginOnlySSO = {
  name: 'okta',
  details: 'some-detail',
  type: 'okta',
  statusCode: IntegrationStatusCode.Running,
  spec: {
    teleportSsoConnector: 'okta-connector',
    scimBearerToken: undefined,
    oktaAppId: 'some-app-id',
    oktaAppName: undefined,
    defaultOwners: undefined,
    orgUrl: 'https://some-okta-url.okta.com',
    error: undefined,
  },
  status: {
    code: IntegrationStatusCode.Running,
    lastRun: new Date(Date.now() - 1000 * 60 * 2),
    errorMessage: undefined,
    details: {
      okta: {
        sso_details: {
          enabled: true,
          app_name: undefined,
          app_id: 'some-app-id',
          okta_group_everyone_mapped_roles: ['some-role'],
        },
      },
    },
  },
};

const renderOktaStatus = async (
  identity = false,
  page?: Exclude<OktaIntegrationStepType, OktaIntegrationStepType.Sso>
) => {
  const ctx = createTeleportContextE();
  cfg.entitlements.Identity = { enabled: identity, limit: 0 };

  render(
    <MemoryRouter
      initialEntries={[cfg.getIntegrationStatusRoute('okta', 'okta', page)]}
    >
      <TeleportContextProvider ctx={ctx}>
        <Routes>
          <Route
            path={`${cfg.routes.integrationStatus}/*`}
            element={<IntegrationStatus />}
          />
        </Routes>
      </TeleportContextProvider>
    </MemoryRouter>
  );
  // Wait for the component to finish rendering
  await act(tick);
};

test('unsupported integration kinds', () => {
  for (const key in IntegrationKind) {
    render(
      <MemoryRouter
        initialEntries={[`/web/integrations/status/${key}/some-name`]}
      >
        <Routes>
          <Route
            path={`${cfg.routes.integrationStatus}/*`}
            element={<IntegrationStatus />}
          />
        </Routes>
      </MemoryRouter>
    );

    expect(
      screen.getByText(`Status for integration type ${key} is not supported`)
    ).toBeInTheDocument();
  }
});

test.each`
  type
  ${'slack'}
  ${'openai'}
  ${'pagerduty'}
  ${'email'}
  ${'jira'}
  ${'discord'}
  ${'mattermost'}
  ${'msteams'}
  ${'opsgenie'}
  ${'servicenow'}
  ${'jamf'}
  ${'datadog'}
  ${'aws-identity-center'}
`('unsupported plugin kind $type', async ({ type }) => {
  server.use(
    http.get('/v1/enterprise/plugin/some-name', () => HttpResponse.json({}))
  );

  const ctx = new TeleportContextE();

  render(
    <TeleportContextProvider ctx={ctx}>
      <MemoryRouter
        initialEntries={[`/web/integrations/status/${type}/some-name`]}
      >
        <Routes>
          <Route
            path={`${cfg.routes.integrationStatus}/*`}
            element={<IntegrationStatus />}
          />
        </Routes>
      </MemoryRouter>
    </TeleportContextProvider>
  );

  expect(
    screen.getByText(`Status for integration type ${type} is not supported`)
  ).toBeInTheDocument();
});
