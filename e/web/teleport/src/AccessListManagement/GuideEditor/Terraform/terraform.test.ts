import { Role, RoleVersion } from 'teleport/services/resources';

import { defaultStandardRoleConditions } from '../Preset/role/conditions/standard';
import { newAccessRole, standardRoleAccessKind } from '../Preset/role/role';
import { makeTerraformRoleBlock } from './terraform';

const roleParams = {
  kind: standardRoleAccessKind as typeof standardRoleAccessKind,
  roleConditions: {
    ...defaultStandardRoleConditions(),
    node_labels: { env: ['prod'] },
  },
};

const originalRole: Role = {
  kind: 'role',
  version: RoleVersion.V7,
  metadata: { name: 'access-standard-acl-preset-abc123' },
  spec: {
    deny: { node_labels: { env: ['legacy'] } },
    options: { max_session_ttl: '8h0m0s' } as any, // partial testing
    allow: { node_labels: { env: ['staging'] } },
  },
};

describe('makeTerraformRoleBlock', () => {
  test('no original, hasAccessDefined: false, returns undefined', () => {
    const result = makeTerraformRoleBlock(roleParams, undefined, false);
    expect(result).toBeUndefined();
  });

  test('no original, hasAccessDefined: true, creates new role', () => {
    const result = makeTerraformRoleBlock(roleParams, undefined, true);
    const expected = newAccessRole(roleParams);
    expect(result.role.spec.allow).toEqual(expected.spec.allow);
  });

  test('original exists, hasAccessDefined: true, replaces allow field and preserves other spec fields', () => {
    const result = makeTerraformRoleBlock(roleParams, originalRole, true);
    const expectedAllow = newAccessRole(roleParams).spec.allow;
    expect(result.role.spec.allow).toEqual(expectedAllow);
    // original's other spec fields are preserved
    expect(result.role.spec.deny).toBe(originalRole.spec.deny);
    expect(result.role.spec.options).toBe(originalRole.spec.options);
  });

  test('original exists, hasAccessDefined: false, allow field is emptied', () => {
    const result = makeTerraformRoleBlock(roleParams, originalRole, false);
    expect(result.role.spec.allow).toEqual({});
    // original is not mutated
    expect(originalRole.spec.allow).toEqual({
      node_labels: { env: ['staging'] },
    });
  });
});
