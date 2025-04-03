/* eslint-disable testing-library/no-node-access */
import { within } from '@testing-library/react';
import { MemoryRouter, Route } from 'react-router';

import {
  act,
  fireEvent,
  render,
  screen,
  tick,
  userEvent,
} from 'design/utils/testing';

import { OktaIntegrationLevel } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { IntegrationStatus } from 'e-teleport/Integrations/IntegrationStatus/IntegrationStatus';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { pluginsService } from 'e-teleport/services/plugins';
import cfg from 'teleport/config';
import {
  IntegrationKind,
  IntegrationStatusCode,
  Plugin,
  PluginOktaSpec,
} from 'teleport/services/integrations';
import {
  PluginOktaSyncStatusCode,
  PluginStatusOkta,
} from 'teleport/services/integrations/oktaStatusTypes';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

const defaultIdentity = cfg.entitlements.Identity;

describe('Okta status', () => {
  afterEach(() => {
    jest.clearAllMocks();
    cfg.entitlements.Identity = defaultIdentity;
  });

  test('does not show unsupported message', async () => {
    jest
      .spyOn(pluginsService, 'fetchPlugin')
      .mockResolvedValue(stubOktaPluginOnlySSO);

    render(
      <MemoryRouter
        initialEntries={[`/web/integrations/status/okta/some-name`]}
      >
        <Route path={cfg.routes.integrationStatus}>
          <IntegrationStatus />
        </Route>
      </MemoryRouter>
    );
    await act(tick);

    expect(
      screen.queryByText(`Status for integration type okta is not supported`)
    ).not.toBeInTheDocument();
    expect(screen.getByText('Okta Integration')).toBeInTheDocument();
  });

  test('renders CTA without Identity entitlement', async () => {
    jest
      .spyOn(pluginsService, 'fetchPlugin')
      .mockResolvedValue(stubOktaPluginOnlySSO);

    await renderOktaStatus();

    expect(
      screen.getByText(stubOktaPluginOnlySSO.spec.orgUrl)
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
    jest
      .spyOn(pluginsService, 'fetchPlugin')
      .mockResolvedValue(stubOktaPluginOnlySSO);

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

    expect(screen.getByText(/edit user sync/i)).toBeInTheDocument();

    jest.spyOn(pluginsService, 'updatePlugin').mockResolvedValueOnce({
      ...stubOktaPluginOnlySSO,
      spec: {
        ...stubOktaPluginOnlySSO.spec,
        enableUserSync: true,
        credentialsInfo: {
          hasConfiguredOauthCredentials: true,
        },
      },
      status: {
        ...stubOktaPluginOnlySSO.status,
        details: {
          ...stubOktaPluginOnlySSO.status.details,
          usersSyncDetails: {
            enabled: true,
            statusCode: PluginOktaSyncStatusCode.Success,
            lastSuccess: new Date(Date.now() - 1000 * 60 * 2),
            lastFailed: undefined,
            numUsers: 20,
            error: undefined,
          },
        },
      },
    } satisfies Plugin<PluginOktaSpec, PluginStatusOkta>);

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
  resourceType: 'plugin',
  name: 'okta',
  kind: 'okta',
  statusCode: IntegrationStatusCode.Running,
  details: 'some-detail',
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
      ssoDetails: {
        enabled: true,
        appName: undefined,
        appId: 'some-app-id',
      },
    },
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

const renderOktaStatus = async (
  identity = false,
  page?: Exclude<OktaIntegrationLevel, OktaIntegrationLevel.SSO>
) => {
  const ctx = createTeleportContextE();
  cfg.entitlements.Identity = { enabled: identity, limit: 0 };

  render(
    <MemoryRouter
      initialEntries={[cfg.getIntegrationStatusRoute('okta', 'okta', page)]}
    >
      <TeleportContextProvider ctx={ctx}>
        <Route path={cfg.routes.integrationStatus}>
          <IntegrationStatus />
        </Route>
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
        <Route path={cfg.routes.integrationStatus}>
          <IntegrationStatus />
        </Route>
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
  ${'entra-id'}
  ${'datadog'}
  ${'aws-identity-center'}
`('unsupported plugin kind $type', async ({ type }) => {
  render(
    <MemoryRouter
      initialEntries={[`/web/integrations/status/${type}/some-name`]}
    >
      <Route path={cfg.routes.integrationStatus}>
        <IntegrationStatus />
      </Route>
    </MemoryRouter>
  );

  expect(
    screen.getByText(`Status for integration type ${type} is not supported`)
  ).toBeInTheDocument();
});
