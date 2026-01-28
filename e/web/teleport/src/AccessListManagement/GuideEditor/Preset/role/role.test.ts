import { defaultRoleVersion } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { optionsWithDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';
import { RoleVersion } from 'teleport/services/resources';

import {
  AwsAccountMap,
  AwsIcAppLabel,
  AwsIcRoleConditions,
  defaultAwsIcRoleConditions,
} from './conditions/awsic';
import {
  defaultStandardRoleConditions,
  StandardRoleConditions,
} from './conditions/standard';
import { getRolePrefixForAccess, getRoleSuffix, newAccessRole } from './role';

test('getRoleSuffix', () => {
  expect(getRoleSuffix('abc123')).toBe('-acl-preset-abc123');
});

describe('getRolePrefixForAccess', () => {
  test.each`
    kind          | expected
    ${'awsic'}    | ${'access-awsic'}
    ${'standard'} | ${'access-standard'}
  `('returns "$expected" for kind "$kind"', ({ kind, expected }) => {
    expect(getRolePrefixForAccess(kind)).toBe(expected);
  });
});

describe('newAccessRole for a awsic role', () => {
  test('creates role with single account assignment - with default version', () => {
    const account: AwsAccountMap = new Map([['acc1', new Set(['ps1'])]]);
    const roleConditions: AwsIcRoleConditions = {
      ...defaultAwsIcRoleConditions(),
      account,
    };

    const result = newAccessRole({
      kind: 'awsic',
      roleConditions,
    });

    expect(result).toEqual({
      kind: 'role',
      version: defaultRoleVersion,
      metadata: { name: 'access-awsic' },
      spec: {
        deny: {},
        options: optionsWithDefaults(defaultRoleVersion),
        allow: {
          app_labels: AwsIcAppLabel,
          account_assignments: [{ account: 'acc1', permission_set: 'ps1' }],
        },
      },
    });
  });

  test('creates role with multiple account assignments - with custom version', () => {
    const account: AwsAccountMap = new Map([
      ['acc1', new Set(['ps1', 'ps2'])],
      ['acc2', new Set(['ps3'])],
    ]);
    const roleConditions: AwsIcRoleConditions = {
      ...defaultAwsIcRoleConditions(),
      account,
    };

    const result = newAccessRole({
      kind: 'awsic',
      roleConditions,
      roleVersion: RoleVersion.V6,
    });

    expect(result).toEqual({
      kind: 'role',
      version: RoleVersion.V6,
      metadata: { name: 'access-awsic' },
      spec: {
        deny: {},
        options: optionsWithDefaults(RoleVersion.V6),
        allow: {
          app_labels: AwsIcAppLabel,
          account_assignments: expect.arrayContaining([
            { account: 'acc1', permission_set: 'ps1' },
            { account: 'acc1', permission_set: 'ps2' },
            { account: 'acc2', permission_set: 'ps3' },
          ]),
        },
      },
    });
  });

  test('returns undefined account_assignments when account is empty', () => {
    const roleConditions = defaultAwsIcRoleConditions();

    const result = newAccessRole({
      kind: 'awsic',
      roleConditions,
    });

    expect(result).toEqual({
      kind: 'role',
      version: defaultRoleVersion,
      metadata: { name: 'access-awsic' },
      spec: {
        deny: {},
        options: optionsWithDefaults(defaultRoleVersion),
        allow: {
          app_labels: AwsIcAppLabel,
          account_assignments: undefined,
        },
      },
    });
  });
});

describe('newAccessRole for a standard role', () => {
  test('with default version', () => {
    const roleConditions: StandardRoleConditions = {
      ...defaultStandardRoleConditions(),
      app_labels: { env: 'prod' },
      db_labels: { team: 'backend' },
    };

    const result = newAccessRole({
      kind: 'standard',
      roleConditions,
    });

    expect(result).toEqual({
      kind: 'role',
      version: defaultRoleVersion,
      metadata: { name: 'access-standard' },
      spec: {
        deny: {},
        options: optionsWithDefaults(defaultRoleVersion),
        allow: {
          app_labels: { env: 'prod' },
          db_labels: { team: 'backend' },
          windows_desktop_labels: {},
          kubernetes_labels: {},
          node_labels: {},
          github_permissions: [],
          aws_role_arns: [],
          azure_identities: [],
          gcp_service_accounts: [],
          mcp: { tools: [] },
          db_names: undefined,
          db_users: undefined,
          kubernetes_groups: [],
          kubernetes_resources: [],
          kubernetes_users: [],
          logins: [],
          windows_desktop_logins: [],
        },
      },
    });
  });

  test('with custom version', () => {
    const roleConditions: StandardRoleConditions = {
      ...defaultStandardRoleConditions(),
      app_labels: { env: 'prod' },
      db_labels: { team: 'backend' },
      db_names: ['dbname'],
      azure_identities: ['azure'],
      mcp: {
        tools: ['tool1'],
      },
    };

    const result = newAccessRole({
      kind: 'standard',
      roleConditions,
      roleVersion: RoleVersion.V5,
    });

    expect(result).toEqual({
      kind: 'role',
      version: RoleVersion.V5,
      metadata: { name: 'access-standard' },
      spec: {
        deny: {},
        options: optionsWithDefaults(RoleVersion.V5),
        allow: {
          app_labels: { env: 'prod' },
          db_labels: { team: 'backend' },
          windows_desktop_labels: {},
          kubernetes_labels: {},
          node_labels: {},
          github_permissions: [],
          aws_role_arns: [],
          azure_identities: ['azure'],
          gcp_service_accounts: [],
          mcp: { tools: ['tool1'] },
          db_names: ['dbname'],
          db_users: undefined,
          kubernetes_groups: [],
          kubernetes_resources: [],
          kubernetes_users: [],
          logins: [],
          windows_desktop_logins: [],
        },
      },
    });
  });
});
