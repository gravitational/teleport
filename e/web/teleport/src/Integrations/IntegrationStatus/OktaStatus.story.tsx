import { Meta, StoryObj } from '@storybook/react-vite';
import { delay, http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter, Route } from 'react-router';

import cfg from 'e-teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { PluginOktaSyncStatusCode } from 'teleport/services/integrations/oktaStatusTypes';
import { storageService } from 'teleport/services/storageService';

import { IntegrationStatus } from './IntegrationStatus';

const defaultIdentity = cfg.oss.entitlements.Identity;
const defaultGetAccessGraphEnabled =
  storageService.getAccessGraphEnabled.bind(storageService);

export default {
  title: 'TeleportE/Integrations/Status/Okta',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.entitlements.Identity = defaultIdentity;
          storageService.getAccessGraphEnabled = defaultGetAccessGraphEnabled;
        };
      }, []);
      return <Story />;
    },
  ],
  render: () => render(cfg.oss.getIntegrationStatusRoute('okta', 'okta')),
} satisfies Meta<typeof IntegrationStatus>;

const basePluginResp = {
  name: 'plugin-name',
  statusCode: PluginOktaSyncStatusCode.Success,
  type: 'okta',
  spec: {
    teleportSsoConnector: 'okta-integration',
    orgUrl: 'https://dev-testing.okta.com',
    defaultOwners: [
      'foo@goteleport.com',
      'george.washington.the.first@cloud.gravitational.io',
    ],
    enableUserSync: true,
    enableAppGroupSync: true,
    enableAccessListSync: true,
    enableSystemLogExport: true,
    credentialsInfo: {
      hasConfiguredOauthCredentials: true,
    },
  },
  status: {
    details: {
      okta: {
        sso_details: {
          enabled: true,
          app_id: 'some-app-id-george-washington-long-app-id',
          app_name:
            'some-app-name-george-washington-testing-long-app-name-lorem-ipsum',
          okta_group_everyone_mapped_roles: ['some-role'],
        },
        app_group_sync_details: {
          last_successful: new Date(Date.now() - 1000 * 60),
          last_failed: null,
          num_apps_synced: 324,
          num_groups_synced: 212,
        },
        users_sync_details: {
          enabled: true,
          last_successful: new Date(Date.now() - 1000 * 60),
          last_failed: null,
          num_users_synced: 130,
        },
        access_lists_sync_details: {
          app_filters: [
            'app*',
            'application-1',
            'application-2',
            'application-3',
            'app4',
            'app5',
            'app6',
          ],
          group_filters: ['group*', 'some-group-1'],
          enabled: true,
          last_successful: new Date(Date.now() - 1000 * 60),
          last_failed: null,
          num_apps_synced: 30,
          num_groups_synced: 2,
        },
        scim_details: { enabled: true },
      },
    },
  },
};

export const WithAllFeaturesEnabled = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.plugin.get, () => HttpResponse.json(basePluginResp)),
      ],
    },
  },
  render: () => {
    cfg.oss.entitlements.Identity = { enabled: true, limit: 0 };
    storageService.getAccessGraphEnabled = () => true;
    return render(cfg.oss.getIntegrationStatusRoute('okta', 'okta'));
  },
} satisfies StoryObj<typeof IntegrationStatus>;

export const WithAllFeaturesEnabledWithoutAppName = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.plugin.get, () =>
          HttpResponse.json({
            ...basePluginResp,
            status: {
              ...basePluginResp.status,
              details: {
                ...basePluginResp.status.details,
                okta: {
                  ...basePluginResp.status.details.okta,
                  sso_details: {
                    ...basePluginResp.status.details.okta.sso_details,
                    app_name: '',
                  },
                },
              },
            },
          })
        ),
      ],
    },
  },
  render: () => {
    cfg.oss.entitlements.Identity = { enabled: true, limit: 0 };
    storageService.getAccessGraphEnabled = () => true;
    return render(cfg.oss.getIntegrationStatusRoute('okta', 'okta'));
  },
} satisfies StoryObj<typeof IntegrationStatus>;

export const Loading = {
  parameters: {
    msw: {
      handlers: [http.get(cfg.api.plugin.get, () => delay('infinite'))],
    },
  },
  render: () => render(cfg.oss.getIntegrationStatusRoute('okta', 'okta')),
} satisfies StoryObj<typeof IntegrationStatus>;

export const Failed = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.plugin.get, () =>
          HttpResponse.json(
            {
              message: 'Whoops, some kind of bad parameter message',
            },
            { status: 404 }
          )
        ),
      ],
    },
  },
  render: () => render(cfg.oss.getIntegrationStatusRoute('okta', 'okta')),
} satisfies StoryObj<typeof IntegrationStatus>;

export const WithSyncErrors = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.plugin.get, () =>
          HttpResponse.json({
            ...basePluginResp,
            spec: {
              ...basePluginResp.spec,
              enableUserSync: true,
              enableAppGroupSync: true,
              enableAccessListSync: true,
              credentialsInfo: {
                hasConfiguredOauthCredentials: true,
                hasSCIMToken: true,
              },
            },
            statusCode: PluginOktaSyncStatusCode.Error,
            status: {
              details: {
                okta: {
                  sso_details: {
                    enabled: false,
                    app_id: 'banana',
                    app_name: 'banana',
                    okta_group_everyone_mapped_roles: ['banana'],
                  },
                  app_group_sync_details: {
                    status_code: PluginOktaSyncStatusCode.Error,
                    error:
                      'lorem ipsum dolores some long error message george washington was the first president of the united states',
                    last_successful: null,
                    last_failed: new Date(Date.now() - 1000 * 60),
                    num_apps_synced: 0,
                    num_groups_synced: 0,
                  },
                  users_sync_details: {
                    enabled: false,
                    status_code: PluginOktaSyncStatusCode.Error,
                    error: 'some error message',
                    last_successful: null,
                    last_failed: new Date(Date.now() - 1000 * 60),
                    numUsers: 0,
                  },
                  access_lists_sync_details: {
                    app_filters: ['app*'],
                    group_filters: [],
                    enabled: false,
                    status_code: PluginOktaSyncStatusCode.Error,
                    error: 'some error message',
                    last_successful: null,
                    last_failed: new Date(Date.now() - 1000 * 60),
                    num_apps_synced: 0,
                    num_groups_synced: 0,
                  },
                },
              },
            },
          })
        ),
      ],
    },
  },
  render: () => {
    cfg.oss.entitlements.Identity = { enabled: true, limit: 0 };
    return render(cfg.oss.getIntegrationStatusRoute('okta', 'okta'));
  },
} satisfies StoryObj<typeof IntegrationStatus>;

export const WithCta = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.plugin.get, () =>
          HttpResponse.json({
            ...basePluginResp,
            spec: {
              orgUrl: basePluginResp.spec.orgUrl,
              teleportSsoConnector: basePluginResp.spec.teleportSsoConnector,
            },
            status: {
              details: {
                okta: {
                  sso_details: basePluginResp.status.details.okta.sso_details,
                  app_group_sync_details: {
                    last_successful: null,
                    last_failed: null,
                  },
                  users_sync_details: {
                    last_successful: null,
                    last_failed: null,
                  },
                  access_lists_sync_details: {
                    last_successful: null,
                    last_failed: null,
                  },
                },
              },
            },
          })
        ),
      ],
    },
  },
  render: () => {
    cfg.oss.entitlements.Identity = { enabled: false, limit: 0 };
    return render(cfg.oss.getIntegrationStatusRoute('okta', 'okta'));
  },
} satisfies StoryObj<typeof IntegrationStatus>;

const render = (pathname: string) => {
  const ctx = createTeleportContext();

  return (
    <MemoryRouter initialEntries={[{ pathname }]}>
      <Route path={cfg.oss.routes.integrationStatus}>
        <ContextProvider ctx={ctx}>
          <IntegrationStatus />
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
};
