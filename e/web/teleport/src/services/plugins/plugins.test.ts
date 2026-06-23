import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth';

import { pluginsService } from './plugins';

beforeEach(() => {
  jest.clearAllMocks();
  jest
    .spyOn(auth, 'getMfaChallengeResponseForAdminAction')
    .mockResolvedValue(undefined);
});

test('create static auth plugins', async () => {
  jest.spyOn(api, 'postFormData').mockResolvedValue({});

  await pluginsService.createStaticAuthPlugin({} as any);

  expect(api.postFormData).toHaveBeenCalledTimes(1);
  expect(api.postFormData).toHaveBeenCalledWith(
    cfg.api.plugin.createStaticAuth,
    {},
    undefined
  );
});

test('make entra id plugin', async () => {
  const lastRun = '2025-12-18T06:31:01.708Z';
  jest.spyOn(api, 'get').mockResolvedValue({
    name: 'entra-id-default',
    type: 'entra-id',
    details:
      'Users and groups will be synchronized from the Entra ID directory',
    statusCode: 2,
    spec: {
      defaultOwners: ['user1'],
      ssoConnectorId: 'entra-id',
      credentialSource: 'ENTRAID_CREDENTIALS_SOURCE_OIDC',
      tenantId: 'abc-tenant',
      entraAppId: 'abc-app',
      groupFilters: {
        id: ['123', '456'],
        nameRegex: ['prod*'],
        excludeId: ['789'],
        excludeNameRegex: ['admin*'],
      },
      accessGraphEnabled: true,
    },
    status: {
      code: 2,
      lastRun: lastRun,
      errorMessage: 'error message',
      lastRawError: 'raw error message',
      details: {
        entra: {
          imported_groups: 12,
          imported_users: 24,
        },
      },
    },
  });
  const resp = await pluginsService.fetchPlugin('entra-id-default');

  expect(resp).toEqual({
    resourceType: 'plugin',
    kind: 'entra-id',
    name: 'entra-id-default',
    spec: {
      defaultOwners: ['user1'],
      ssoConnectorId: 'entra-id',
      credentialSource: 'ENTRAID_CREDENTIALS_SOURCE_OIDC',
      tenantId: 'abc-tenant',
      entraAppId: 'abc-app',
      groupFilters: {
        id: ['123', '456'],
        nameRegex: ['prod*'],
        excludeId: ['789'],
        excludeNameRegex: ['admin*'],
      },
      accessGraphEnabled: true,
    },
    credentials: undefined,
    details:
      'Users and groups will be synchronized from the Entra ID directory',
    statusCode: 2,
    status: {
      code: 2,
      lastRun: new Date(lastRun),
      errorMessage: 'error message',
      lastRawError: 'raw error message',
      details: {
        imported_groups: 12,
        imported_users: 24,
      },
    },
  });
});
