import { MinimumRoleVersionSupported } from 'e-teleport/AccessListManagement/GuideEditor/useGuideEditor';
import { optionsWithDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';

import {
  AwsIcAppLabel,
  convertAwsAccountMapToRoleType,
  extractAwsIcRoleConditionsFromRole,
} from './awsic';

describe('extractAwsIcRoleConditionsFromRole', () => {
  test('extracts single account with single permission set', () => {
    const role = createRole([{ account: 'acc1', permission_set: 'ps1' }]);

    expect(extractAwsIcRoleConditionsFromRole(role)).toEqual({
      labels: AwsIcAppLabel,
      account: new Map([['acc1', new Set(['ps1'])]]),
    });
  });

  test('extracts single account with multiple permission sets', () => {
    const role = createRole([
      { account: 'acc1', permission_set: 'ps1' },
      { account: 'acc1', permission_set: 'ps2' },
      { account: 'acc1', permission_set: 'ps3' },
    ]);

    expect(extractAwsIcRoleConditionsFromRole(role)).toEqual({
      labels: AwsIcAppLabel,
      account: new Map([['acc1', new Set(['ps1', 'ps2', 'ps3'])]]),
    });
  });

  test('extracts multiple accounts with various permission sets', () => {
    const role = createRole([
      { account: 'acc1', permission_set: 'ps1' },
      { account: 'acc1', permission_set: 'ps2' },
      { account: 'acc2', permission_set: 'ps3' },
      { account: 'acc3', permission_set: 'ps4' },
      { account: 'acc3', permission_set: 'ps5' },
    ]);

    expect(extractAwsIcRoleConditionsFromRole(role)).toEqual({
      labels: AwsIcAppLabel,
      account: new Map([
        ['acc1', new Set(['ps1', 'ps2'])],
        ['acc2', new Set(['ps3'])],
        ['acc3', new Set(['ps4', 'ps5'])],
      ]),
    });
  });

  test('extracts empty account map when no assignments', () => {
    const role = createRole([]);

    expect(extractAwsIcRoleConditionsFromRole(role)).toEqual({
      labels: AwsIcAppLabel,
      account: new Map(),
    });
  });

  test('uses role version from the role', () => {
    const role = createRole([{ account: 'acc1', permission_set: 'ps1' }]);

    expect(extractAwsIcRoleConditionsFromRole(role)).toEqual({
      labels: AwsIcAppLabel,
      account: new Map([['acc1', new Set(['ps1'])]]),
    });
  });

  test('handles duplicate permission sets for same account', () => {
    const role = createRole([
      { account: 'acc1', permission_set: 'ps1' },
      { account: 'acc1', permission_set: 'ps1' },
    ]);

    expect(extractAwsIcRoleConditionsFromRole(role)).toEqual({
      labels: AwsIcAppLabel,
      account: new Map([['acc1', new Set(['ps1'])]]),
    });
  });
});

describe('convertAwsAccountMapToRoleType', () => {
  test('returns undefined when account map is empty', () => {
    expect(convertAwsAccountMapToRoleType(new Map())).toBeUndefined();
  });

  test('converts single account with single permission set', () => {
    const account = new Map([['acc1', new Set(['ps1'])]]);

    expect(convertAwsAccountMapToRoleType(account)).toEqual([
      { account: 'acc1', permission_set: 'ps1' },
    ]);
  });

  test('converts single account with multiple permission sets', () => {
    const account = new Map([['acc1', new Set(['ps1', 'ps2', 'ps3'])]]);

    expect(convertAwsAccountMapToRoleType(account)).toEqual(
      expect.arrayContaining([
        { account: 'acc1', permission_set: 'ps1' },
        { account: 'acc1', permission_set: 'ps2' },
        { account: 'acc1', permission_set: 'ps3' },
      ])
    );
  });

  test('converts multiple accounts with various permission sets', () => {
    const account = new Map([
      ['acc1', new Set(['ps1', 'ps2'])],
      ['acc2', new Set(['ps3'])],
      ['acc3', new Set(['ps4', 'ps5'])],
    ]);

    expect(convertAwsAccountMapToRoleType(account)).toEqual(
      expect.arrayContaining([
        { account: 'acc1', permission_set: 'ps1' },
        { account: 'acc1', permission_set: 'ps2' },
        { account: 'acc2', permission_set: 'ps3' },
        { account: 'acc3', permission_set: 'ps4' },
        { account: 'acc3', permission_set: 'ps5' },
      ])
    );
  });
});

const createRole = (
  accountAssignments: { account: string; permission_set: string }[]
) => ({
  kind: 'role' as const,
  version: MinimumRoleVersionSupported,
  metadata: { name: 'test-role' },
  spec: {
    allow: {
      app_labels: AwsIcAppLabel,
      account_assignments: accountAssignments,
    },
    deny: {},
    options: optionsWithDefaults(MinimumRoleVersionSupported),
  },
});
