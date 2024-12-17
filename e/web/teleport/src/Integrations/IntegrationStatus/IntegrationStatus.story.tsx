import { useEffect } from 'react';
import { MemoryRouter, Route } from 'react-router';
import { http, HttpResponse, delay } from 'msw';
import { PluginOktaSyncStatusCode } from 'teleport/services/integrations/oktaStatusTypes';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import cfg from 'e-teleport/config';

import { IntegrationStatus } from './IntegrationStatus';

const defaultIsCloud = cfg.oss.isCloud;
const defaultAccessList = cfg.oss.entitlements.AccessLists;
const defaultScim = cfg.oss.entitlements.OktaSCIM;
const defaultUserSync = cfg.oss.entitlements.OktaUserSync;

export default {
  title: 'TeleportE/Integrations/Status/Okta',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isCloud = defaultIsCloud;
          cfg.oss.entitlements.AccessLists = defaultAccessList;
          cfg.oss.entitlements.OktaSCIM = defaultScim;
          cfg.oss.entitlements.OktaUserSync = defaultUserSync;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const WithAllFeaturesEnabled = () => {
  cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };
  cfg.oss.entitlements.OktaSCIM = { enabled: true, limit: 0 };
  cfg.oss.entitlements.OktaUserSync = { enabled: true, limit: 0 };
  return render(cfg.oss.getIntegrationStatusRoute('okta', 'some-id'));
};
WithAllFeaturesEnabled.parameters = {
  msw: {
    handlers: [
      http.get(cfg.api.pluginPath, () =>
        HttpResponse.json({
          name: 'plugin-name',
          statusCode: 1, // running
          type: 'okta',
          spec: {
            teleportSsoConnector: 'okta-integration',
            orgUrl: 'https://dev-testing.okta.com',
            defaultOwners: [
              'foo@goteleport.com',
              'george.washington.the.first@cloud.gravitational.io',
            ],
          },
          status: {
            details: {
              okta: {
                sso_details: {
                  enabled: true,
                  app_id: 'some-app-id-george-washington-long-app-id',
                  app_name:
                    'some-app-name-george-washington-testing-long-app-name-lorem-ipsum',
                },
                app_group_sync_details: {
                  last_successful: null,
                  last_failed: new Date(),
                  num_apps_synced: 324,
                  num_groups_synced: 212,
                },
                users_sync_details: {
                  enabled: true,
                  last_successful: null,
                  last_failed: null,
                  numUsers: 130,
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
                  last_successful: new Date(),
                  last_failed: new Date(),
                  num_apps_synced: 30,
                  num_groups_synced: 2,
                },
                scim_details: { enabled: true },
              },
            },
          },
        })
      ),
    ],
  },
};

export const WithAllFeaturesEnabledWithoutAppName = () => {
  cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };
  cfg.oss.entitlements.OktaSCIM = { enabled: true, limit: 0 };
  cfg.oss.entitlements.OktaUserSync = { enabled: true, limit: 0 };
  return render(cfg.oss.getIntegrationStatusRoute('okta', 'some-id'));
};
WithAllFeaturesEnabledWithoutAppName.parameters = {
  msw: {
    handlers: [
      http.get(cfg.api.pluginPath, () =>
        HttpResponse.json({
          name: 'plugin-name',
          statusCode: 1, // running
          type: 'okta',
          spec: {
            teleportSsoConnector: 'okta-integration',
            orgUrl: 'https://dev-testing.okta.com',
            defaultOwners: [
              'foo@goteleport.com',
              'george.washington.the.first@cloud.gravitational.io',
            ],
          },
          status: {
            details: {
              okta: {
                sso_details: {
                  enabled: true,
                  app_id: 'some-app-id-george-washington-long-app-id',
                  app_name: '',
                },
                app_group_sync_details: {
                  last_successful: null,
                  last_failed: new Date(),
                  num_apps_synced: 324,
                  num_groups_synced: 212,
                },
                users_sync_details: {
                  enabled: true,
                  last_successful: null,
                  last_failed: null,
                  numUsers: 130,
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
                  last_successful: new Date(),
                  last_failed: new Date(),
                  num_apps_synced: 30,
                  num_groups_synced: 2,
                },
                scim_details: { enabled: true },
              },
            },
          },
        })
      ),
    ],
  },
};

export const Loading = () =>
  render(cfg.oss.getIntegrationStatusRoute('okta', 'some-id'));
Loading.parameters = {
  msw: {
    handlers: [http.get(cfg.api.pluginPath, () => delay('infinite'))],
  },
};

export const Failed = () =>
  render(cfg.oss.getIntegrationStatusRoute('okta', 'some-id'));
Failed.parameters = {
  msw: {
    handlers: [
      http.get(cfg.api.pluginPath, () =>
        HttpResponse.json(
          {
            message: 'Whoops, some kind of bad paramter message',
          },
          { status: 404 }
        )
      ),
    ],
  },
};

export const WithSyncErrors = () => {
  cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };
  cfg.oss.entitlements.OktaSCIM = { enabled: true, limit: 0 };
  cfg.oss.entitlements.OktaUserSync = { enabled: true, limit: 0 };
  return render(cfg.oss.getIntegrationStatusRoute('okta', 'some-id'));
};
WithSyncErrors.parameters = {
  msw: {
    handlers: [
      http.get(cfg.api.pluginPath, () =>
        HttpResponse.json({
          name: 'plugin-name',
          statusCode: 2, // error
          type: 'okta',
          spec: {
            default_owners: ['admin', 'root'],
            orgUrl: 'https://dev-testing.okta.com',
            teleportSsoConnector: 'some-connector-name',
          },
          status: {
            details: {
              okta: {
                sso_details: {
                  enabled: false,
                  appId: 'banana',
                  appName: 'banana',
                },
                app_group_sync_details: {
                  status_code: PluginOktaSyncStatusCode.Error,
                  error:
                    'lorem ipsum dolores some long error message george washington was the first president of the united states',
                  last_successful: new Date('0001-01-01T00:00:00Z'),
                  last_failed: new Date('2020-01-01T00:00:00Z'),
                  num_apps_synced: 0,
                  num_groups_synced: 0,
                },
                users_sync_details: {
                  enabled: false,
                  status_code: PluginOktaSyncStatusCode.Error,
                  error: 'some error message',
                  last_successful: new Date('0001-01-01T00:00:00Z'),
                  last_failed: new Date('2020-01-01T00:00:00Z'),
                  numUsers: 0,
                },
                access_lists_sync_details: {
                  app_filters: ['app*'],
                  group_filters: [],
                  enabled: false,
                  status_code: PluginOktaSyncStatusCode.Error,
                  error: 'some error message',
                  last_successful: new Date('0001-01-01T00:00:00Z'),
                  last_failed: new Date('2020-01-01T00:00:00Z'),
                  num_apps_synced: 0,
                  num_groups_synced: 0,
                },
                scim_details: { enabled: false },
              },
            },
          },
        })
      ),
    ],
  },
};

export const WithCta = () => {
  return render(cfg.oss.getIntegrationStatusRoute('okta', 'some-id'));
};
WithCta.parameters = {
  msw: {
    handlers: [
      http.get(cfg.api.pluginPath, () =>
        HttpResponse.json({
          name: 'plugin-name',
          statusCode: 1, // error
          type: 'okta',
          spec: {
            orgUrl: 'https://dev-testing.okta.com',
            teleportSsoConnector: 'some-connector-name',
          },
          status: {
            details: {
              okta: {
                sso_details: {
                  enabled: true,
                  appId: 'some-app-id',
                  appName: 'some-app-name',
                },
                app_group_sync_details: {
                  status_code: PluginOktaSyncStatusCode.Error,
                  error: 'some error message',
                  last_successful: new Date(),
                  last_failed: new Date(),
                  num_apps_synced: 324,
                  num_groups_synced: 212,
                },
                users_sync_details: {
                  enabled: true,
                  last_successful: new Date(),
                  last_failed: new Date(),
                  status_code: PluginOktaSyncStatusCode.Error,
                  error: 'some error message',
                  numUsers: 130,
                },
                access_lists_sync_details: {
                  app_filters: ['app*'],
                  group_filters: [],
                  enabled: false,
                  last_successful: new Date(),
                  last_failed: new Date(),
                  status_code: PluginOktaSyncStatusCode.Error,
                  error: 'some error message',
                  num_apps_synced: 30,
                  num_groups_synced: 2,
                },
                scim_details: { enabled: false },
              },
            },
          },
        })
      ),
    ],
  },
};

function render(pathname) {
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
}
