/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { AwsOidcPolicyPreset } from 'teleport/services/integrations';

import cfg, {
  UrlAwsConfigureIamScriptParams,
  UrlAwsOidcConfigureIdp,
  UrlDeployServiceIamConfigureScriptParams,
} from './config';
import { IntegrationTag } from './Integrations/Enroll/Shared';

test('getDeployServiceIamConfigureScriptPath formatting', () => {
  const params: UrlDeployServiceIamConfigureScriptParams = {
    integrationName: 'int-name',
    region: 'us-east-1',
    awsOidcRoleArn: 'oidc-arn',
    taskRoleArn: 'task-arn',
    accountID: '123456789012',
  };
  const base =
    'http://localhost/v1/webapi/scripts/integrations/configure/deployservice-iam.sh?';
  const expected = `integrationName=${'int-name'}&awsRegion=${'us-east-1'}&role=${'oidc-arn'}&taskRole=${'task-arn'}&awsAccountID=${'123456789012'}`;
  expect(cfg.getDeployServiceIamConfigureScriptUrl(params)).toBe(
    `${base}${expected}`
  );
});

test('getAwsOidcConfigureIdpScriptUrl formatting, without s3 fields', () => {
  const params: UrlAwsOidcConfigureIdp = {
    integrationName: 'int-name',
    roleName: 'role-arn',
    policyPreset: AwsOidcPolicyPreset.Unspecified,
  };
  const base =
    'http://localhost/v1/webapi/scripts/integrations/configure/awsoidc-idp.sh?';
  const expected = `integrationName=int-name&role=role-arn&policyPreset=`;
  expect(cfg.getAwsOidcConfigureIdpScriptUrl(params)).toBe(
    `${base}${expected}`
  );
});

test('getAwsIamConfigureScriptAppAccessUrl formatting', () => {
  const params: Omit<UrlAwsConfigureIamScriptParams, 'region'> = {
    iamRoleName: 'role-arn',
    accountID: '123456789012',
  };
  const base =
    'http://localhost/v1/webapi/scripts/integrations/configure/aws-app-access-iam.sh?';
  const expected = `role=role-arn&awsAccountID=123456789012`;
  expect(cfg.getAwsIamConfigureScriptAppAccessUrl(params)).toBe(
    `${base}${expected}`
  );
});

test('getIntegrationsEnroll appends tags', () => {
  const tags: IntegrationTag[] = ['devicetrust', 'idp'];
  const url = new URL(
    'https://example.com' + cfg.getIntegrationsEnrollRoute({ tags })
  );
  expect(url.searchParams.getAll('tags')).toEqual(tags);
});

test('getIntegrationsEnroll without extra params', () => {
  expect(cfg.getIntegrationsEnrollRoute()).toEqual('/web/integrations/new');
});

test('getSsoUrl', () => {
  const providerUrl =
    '/v1/webapi/oidc/login/web?connector_id=:providerName&login_hint=:loginHint?&redirect_url=:redirect';
  expect(
    cfg.getSsoUrl(providerUrl, 'keycloak', 'example.com', undefined)
  ).toEqual(
    'http://localhost/v1/webapi/oidc/login/web?connector_id=keycloak&redirect_url=example.com'
  );
  expect(
    cfg.getSsoUrl(providerUrl, 'keycloak', 'example.com', 'user@example.com')
  ).toEqual(
    'http://localhost/v1/webapi/oidc/login/web?connector_id=keycloak&login_hint=user%40example.com&redirect_url=example.com'
  );
  expect(
    cfg.getSsoUrl(
      providerUrl,
      'keycloak',
      'example.com?a=b&c=d',
      'user@example.com'
    )
  ).toEqual(
    'http://localhost/v1/webapi/oidc/login/web?connector_id=keycloak&login_hint=user%40example.com&redirect_url=example.com%3Fa%3Db%26c%3Dd'
  );
});

test('getUsersUrlV2 encodes params', () => {
  expect(
    cfg.getUsersUrlV2({
      startKey: 'next=1&offset=2',
      search: 'user@example.com / admin',
      limit: 25,
    })
  ).toEqual(
    '/v2/webapi/users?startKey=next%3D1%26offset%3D2&search=user%40example.com%20%2F%20admin&limit=25'
  );
});

test('getUsersUrlV2 clears optional params', () => {
  expect(cfg.getUsersUrlV2()).toEqual(
    '/v2/webapi/users?startKey=&search=&limit='
  );
});

test('getClusterEventsUrl does not corrupt startKey when start is set', () => {
  const url = cfg.getClusterEventsUrlV2('root', {
    start: '2025-01-01T00:00:00.000Z',
    end: '',
    startKey: 'next=1&offset=2',
  });

  expect(url).toContain('from=2025-01-01T00%3A00%3A00.000Z');
  expect(url).toContain('startKey=next%3D1%26offset%3D2');
  expect(url).not.toContain('00.000ZKey?');
});

test('getAppLauncherRoute encodes slash in AWS role ARN', () => {
  const url = cfg.getAppLauncherRoute({
    fqdn: 'app.example.com',
    clusterId: 'cluster1',
    publicAddr: 'app.example.com',
    arn: 'arn:aws:iam::123456789012:role/my-role',
  });
  expect(url).toContain('role%2Fmy-role');
  expect(url).not.toContain('role/my-role');
});

test('getAppLauncherRoute encodes multi-level ARN path', () => {
  const url = cfg.getAppLauncherRoute({
    fqdn: 'app.example.com',
    clusterId: 'cluster1',
    publicAddr: 'app.example.com',
    arn: 'arn:aws:iam::123456789012:role/path/to/my-role',
  });
  expect(url).toContain('role%2Fpath%2Fto%2Fmy-role');
});

test('getAppLauncherRoute without ARN leaves route unchanged', () => {
  const url = cfg.getAppLauncherRoute({
    fqdn: 'app.example.com',
    clusterId: 'cluster1',
    publicAddr: 'app.example.com',
  });
  expect(url).toBe('/web/launch/app.example.com/cluster1/app.example.com');
});

describe('MFA helpers', () => {
  const original = {
    second_factor: cfg.auth.second_factor,
    second_factors: cfg.auth.second_factors,
  };
  afterEach(() => {
    cfg.auth.second_factor = original.second_factor;
    cfg.auth.second_factors = original.second_factors;
  });

  describe('secondFactors()', () => {
    test.each`
      second_factors                | expected
      ${['webauthn']}               | ${['webauthn']}
      ${['sso']}                    | ${['sso']}
      ${['otp', 'webauthn', 'sso']} | ${['otp', 'webauthn', 'sso']}
    `('returns $second_factors directly', ({ second_factors, expected }) => {
      cfg.auth.second_factors = second_factors;
      expect(cfg.secondFactors()).toEqual(expected);
    });

    test.each`
      second_factor | expected
      ${'webauthn'} | ${['webauthn']}
      ${'otp'}      | ${['otp']}
      ${'on'}       | ${['otp', 'webauthn']}
      ${'optional'} | ${['otp', 'webauthn']}
      ${'off'}      | ${[]}
    `(
      'derives from legacy second_factor=$second_factor when second_factors is empty',
      ({ second_factor, expected }) => {
        cfg.auth.second_factors = [];
        cfg.auth.second_factor = second_factor;
        expect(cfg.secondFactors()).toEqual(expected);
      }
    );
  });

  describe('isAdminActionMfaEnforced()', () => {
    test.each`
      second_factors         | expected
      ${['webauthn']}        | ${true}
      ${['sso']}             | ${true}
      ${['webauthn', 'sso']} | ${true}
      ${['otp']}             | ${false}
      ${['otp', 'webauthn']} | ${false}
    `(
      'second_factors=$second_factors → $expected',
      ({ second_factors, expected }) => {
        cfg.auth.second_factors = second_factors;
        expect(cfg.isAdminActionMfaEnforced()).toBe(expected);
      }
    );

    test.each`
      second_factor | expected
      ${'webauthn'} | ${true}
      ${'otp'}      | ${false}
      ${'on'}       | ${false}
      ${'optional'} | ${false}
    `(
      'legacy second_factor=$second_factor with empty second_factors → $expected',
      ({ second_factor, expected }) => {
        cfg.auth.second_factors = [];
        cfg.auth.second_factor = second_factor;
        expect(cfg.isAdminActionMfaEnforced()).toBe(expected);
      }
    );

    test('legacy second_factor=off with empty second_factors is undefined (SSO-only ambiguous)', () => {
      cfg.auth.second_factors = [];
      cfg.auth.second_factor = 'off';
      expect(cfg.isAdminActionMfaEnforced()).toBeUndefined();
    });
  });

  describe('isMfaUserConfigurable()', () => {
    test.each`
      second_factors                | expected
      ${['webauthn']}               | ${true}
      ${['otp']}                    | ${true}
      ${['otp', 'webauthn', 'sso']} | ${true}
      ${['sso']}                    | ${false}
      ${[]}                         | ${false}
    `(
      'second_factors=$second_factors → $expected',
      ({ second_factors, expected }) => {
        cfg.auth.second_factors = second_factors;
        cfg.auth.second_factor = 'off';
        expect(cfg.isMfaUserConfigurable()).toBe(expected);
      }
    );
  });
});

test('getRoleUrl listv2 encodes query params', () => {
  expect(
    cfg.getRoleUrl({
      action: 'listv2',
      params: {
        startKey: 'next=1&offset=2',
        search: 'role:admin@example.com',
        limit: 50,
        includeSystemRoles: 'yes',
        includeObject: 'yes',
      },
    })
  ).toEqual(
    '/v2/webapi/roles?startKey=next%3D1%26offset%3D2&search=role%3Aadmin%40example.com&limit=50&includeSystemRoles=yes&includeObject=yes'
  );
});
