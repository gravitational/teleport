import { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';
import { defaultRoleVersion } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { optionsWithDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';
import { Role, RoleConditions } from 'teleport/services/resources';

import {
  awsIcRole,
  awsIcRoleEmptyAccess,
  awsIcRoleFullyFleshedOut,
  awsIcRoleWithDeny,
  awsIcRoleWithoutAccountAssignments,
  awsIcRoleWithoutLabels,
  awsIcRoleWithUnknownFields,
  awsIcRoleWithUnsupportedVersion,
  internalAccessListPresetLabelKey,
  standardRole,
  standardRoleEmptyAccess,
  standardRoleFullyFleshedOut,
  standardRoleWithDeny,
  standardRoleWithUnknownFields,
  standardRoleWithUnsupportedVersion,
  testAccessListId,
} from '../TestHelper/roles';
import { validateQueriedRoles } from './role';

const standardRoleWithMissingLabel: Role = {
  ...standardRole,
  metadata: { ...standardRole.metadata, labels: {} },
};

// Creates a minimal AccessListModified for testing.
function makeAccessList(
  overrides: Partial<{
    id: string;
    preset: AccessListPreset;
    memberRolesGranted: string[];
  }> = {}
): AccessListModified {
  const {
    id = testAccessListId,
    preset = 'long-term',
    memberRolesGranted = [],
  } = overrides;

  return {
    id,
    preset,
    grants: { roles: memberRolesGranted, traits: {} },
  } as AccessListModified;
}

// Creates a requester role for short-term access.
const requesterRoleName = `requester-acl-preset-${testAccessListId}`;
function makeRequesterRole(searchAsRoles: string[]): Role {
  return {
    kind: 'role',
    version: defaultRoleVersion,
    metadata: {
      name: requesterRoleName,
      labels: { [internalAccessListPresetLabelKey]: testAccessListId },
    },
    spec: {
      allow: {
        request: { search_as_roles: searchAsRoles },
      } as Role['spec']['allow'],
      deny: {},
      options: optionsWithDefaults(defaultRoleVersion),
    },
  };
}

describe('no-access-defined regardless of preset', () => {
  test.each`
    desc                                    | gotRoles                                           | memberRolesGranted                                                             | expectedResult
    ${'no member grants assigned - single'} | ${[standardRole]}                                  | ${[]}                                                                          | ${{ status: 'no-access-defined' }}
    ${'no member grants assigned - multi'}  | ${[standardRole, awsIcRole]}                       | ${[]}                                                                          | ${{ status: 'no-access-defined' }}
    ${'standard role exists but no access'} | ${[standardRoleEmptyAccess]}                       | ${[standardRoleEmptyAccess.metadata.name]}                                     | ${{ status: 'no-access-defined', standardRole: standardRoleEmptyAccess }}
    ${'awsIc role exists but no access'}    | ${[awsIcRoleEmptyAccess]}                          | ${[awsIcRoleEmptyAccess.metadata.name]}                                        | ${{ status: 'no-access-defined', awsIcRole: awsIcRoleEmptyAccess }}
    ${'both roles exists but no access'}    | ${[awsIcRoleEmptyAccess, standardRoleEmptyAccess]} | ${[awsIcRoleEmptyAccess.metadata.name, standardRoleEmptyAccess.metadata.name]} | ${{ status: 'no-access-defined', awsIcRole: awsIcRoleEmptyAccess, standardRole: standardRoleEmptyAccess }}
  `(
    'returns no-access-defined when $desc',
    ({ gotRoles, memberRolesGranted, expectedResult }) => {
      const result = validateQueriedRoles({
        gotRoles,
        accessList: makeAccessList({ memberRolesGranted }),
      });

      expect(result).toEqual(expectedResult);
    }
  );
});

describe('test validating for long-term preset', () => {
  test.each`
    desc                                 | roles                            | expectedStandard               | expectedAwsIc
    ${'standard role minimal'}           | ${[standardRole]}                | ${standardRole}                | ${undefined}
    ${'standard role fully fleshed out'} | ${[standardRoleFullyFleshedOut]} | ${standardRoleFullyFleshedOut} | ${undefined}
    ${'awsIc role'}                      | ${[awsIcRole]}                   | ${undefined}                   | ${awsIcRole}
    ${'awsIc role fully fleshed out'}    | ${[awsIcRoleFullyFleshedOut]}    | ${undefined}                   | ${awsIcRoleFullyFleshedOut}
    ${'both standard and awsIc'}         | ${[standardRole, awsIcRole]}     | ${standardRole}                | ${awsIcRole}
  `(
    'returns valid-roles for $desc',
    ({ roles, expectedStandard, expectedAwsIc }) => {
      const result = validateQueriedRoles({
        gotRoles: roles,
        accessList: makeAccessList({
          preset: 'long-term',
          memberRolesGranted: roles.map((r: Role) => r.metadata.name),
        }),
      });

      expect(result).toEqual({
        status: 'valid-roles',
        standardRole: expectedStandard,
        awsIcRole: expectedAwsIc,
      });
    }
  );

  test.each`
    desc                             | gotRoles                          | memberRolesGranted
    ${'granted role not in queried'} | ${[standardRole]}                 | ${[standardRole.metadata.name, 'unknown-role']}
    ${'role missing internal label'} | ${[standardRoleWithMissingLabel]} | ${[standardRoleWithMissingLabel.metadata.name]}
    ${'no roles returned'}           | ${[]}                             | ${[standardRole.metadata.name]}
  `('returns unknown-roles when $desc', ({ gotRoles, memberRolesGranted }) => {
    const result = validateQueriedRoles({
      gotRoles,
      accessList: makeAccessList({
        preset: 'long-term',
        memberRolesGranted,
      }),
    });

    expect(result.status).toBe('unknown-roles');
  });
});

describe('test validating for short-term preset', () => {
  test.each`
    desc                                 | gotRoles                                                                                               | expectedStandard               | expectedAwsIc
    ${'standard role'}                   | ${[standardRole, makeRequesterRole([standardRole.metadata.name])]}                                     | ${standardRole}                | ${undefined}
    ${'standard role fully fleshed out'} | ${[standardRoleFullyFleshedOut, makeRequesterRole([standardRoleFullyFleshedOut.metadata.name])]}       | ${standardRoleFullyFleshedOut} | ${undefined}
    ${'awsIc role'}                      | ${[awsIcRole, makeRequesterRole([awsIcRole.metadata.name])]}                                           | ${undefined}                   | ${awsIcRole}
    ${'awsic role fully fleshed out'}    | ${[awsIcRoleFullyFleshedOut, makeRequesterRole([awsIcRoleFullyFleshedOut.metadata.name])]}             | ${undefined}                   | ${awsIcRoleFullyFleshedOut}
    ${'both standard and awsIc'}         | ${[standardRole, awsIcRole, makeRequesterRole([standardRole.metadata.name, awsIcRole.metadata.name])]} | ${standardRole}                | ${awsIcRole}
  `(
    'returns valid-roles for $desc',
    ({ gotRoles, expectedStandard, expectedAwsIc }) => {
      const result = validateQueriedRoles({
        gotRoles,
        accessList: makeAccessList({
          preset: 'short-term',
          memberRolesGranted: [requesterRoleName],
        }),
      });

      expect(result).toEqual({
        status: 'valid-roles',
        standardRole: expectedStandard,
        awsIcRole: expectedAwsIc,
      });
    }
  );

  test.each`
    desc                             | gotRoles                                                                           | memberRolesGranted
    ${'no roles returned'}           | ${[]}                                                                              | ${[standardRole.metadata.name]}
    ${'no requester role found'}     | ${[standardRole]}                                                                  | ${['requester-acl-preset-ABCD']}
    ${'multiple grants'}             | ${[standardRole, makeRequesterRole([standardRole.metadata.name])]}                 | ${[`requester-acl-preset-${testAccessListId}`, 'unknown-role']}
    ${'search_as_roles has unknown'} | ${[standardRole, makeRequesterRole([standardRole.metadata.name, 'unknown-role'])]} | ${[`requester-acl-preset-${testAccessListId}`]}
  `('returns unknown-roles when $desc', ({ gotRoles, memberRolesGranted }) => {
    const result = validateQueriedRoles({
      gotRoles,
      accessList: makeAccessList({
        preset: 'short-term',
        memberRolesGranted,
      }),
    });

    expect(result.status).toBe('unknown-roles');
  });
});

describe('short-term preset requester role validation', () => {
  function makeRequesterRoleWithAllow(allow: Partial<RoleConditions>): Role {
    return {
      kind: 'role',
      version: defaultRoleVersion,
      metadata: {
        name: requesterRoleName,
        labels: { [internalAccessListPresetLabelKey]: testAccessListId },
      },
      spec: {
        allow,
        deny: {},
        options: optionsWithDefaults(defaultRoleVersion),
      },
    };
  }

  const roleName = standardRole.metadata.name;

  test.each`
    desc                             | allow
    ${'has app_labels'}              | ${{ request: { search_as_roles: [roleName] }, app_labels: { env: 'prod' } }}
    ${'has logins'}                  | ${{ request: { search_as_roles: [roleName] }, logins: ['root'] }}
    ${'has unknown field'}           | ${{ request: { search_as_roles: [roleName] }, something: { abc: 'abc' } }}
    ${'has request.roles'}           | ${{ request: { search_as_roles: [roleName], roles: ['some-role'] } }}
    ${'has request.thresholds'}      | ${{ request: { search_as_roles: [roleName], thresholds: [{ approve: 1, deny: 1 }] } }}
    ${'has request.claims_to_roles'} | ${{ request: { search_as_roles: [roleName], claims_to_roles: [{ claim: 'group', value: 'admin', roles: ['admin'] }] } }}
  `(
    'returns unsupported-role-fields when requester role $desc',
    ({ allow }) => {
      const requesterRole = makeRequesterRoleWithAllow(allow);
      const result = validateQueriedRoles({
        gotRoles: [standardRole, requesterRole],
        accessList: makeAccessList({
          preset: 'short-term',
          memberRolesGranted: [requesterRoleName],
        }),
      });

      expect(result.status).toBe('unsupported-role-fields');
    }
  );

  test.each`
    desc                                  | allow
    ${'only has request.search_as_roles'} | ${{ request: { search_as_roles: [roleName] } }}
    ${'has empty unsupported fields'}     | ${{ request: { search_as_roles: [roleName] }, app_labels: {}, logins: [] }}
  `('returns valid-roles when requester role $desc', ({ allow }) => {
    const requesterRole = makeRequesterRoleWithAllow(
      allow as Role['spec']['allow']
    );
    const result = validateQueriedRoles({
      gotRoles: [standardRole, requesterRole],
      accessList: makeAccessList({
        preset: 'short-term',
        memberRolesGranted: [requesterRoleName],
      }),
    });

    expect(result.status).toBe('valid-roles');
  });
});

describe('unsupported-role-fields status', () => {
  test.each`
    desc                                        | role
    ${'standard role with deny fields'}         | ${standardRoleWithDeny}
    ${'awsIc role with deny fields'}            | ${awsIcRoleWithDeny}
    ${'standard role with bad version'}         | ${standardRoleWithUnsupportedVersion}
    ${'awsIc role with bad version'}            | ${awsIcRoleWithUnsupportedVersion}
    ${'standard role with unknown fields'}      | ${standardRoleWithUnknownFields}
    ${'awsIc role with unknown fields'}         | ${awsIcRoleWithUnknownFields}
    ${'awsIc role without account_assignments'} | ${awsIcRoleWithoutAccountAssignments}
    ${'awsIc role without app_labels'}          | ${awsIcRoleWithoutLabels}
  `('returns unsupported-role-fields for $desc', ({ role }) => {
    const result = validateQueriedRoles({
      gotRoles: [role],
      accessList: makeAccessList({
        memberRolesGranted: [role.metadata.name],
      }),
    });

    expect(result.status).toBe('unsupported-role-fields');
  });
});

describe('isStandardRoleKnownFieldUnsupported interpolation checks', () => {
  function makeStandardRoleWithLabels(
    labelField: string,
    labels: Record<string, string | string[]>
  ): Role {
    return {
      kind: 'role',
      version: defaultRoleVersion,
      metadata: {
        name: `access-standard-acl-preset-${testAccessListId}`,
        labels: { [internalAccessListPresetLabelKey]: testAccessListId },
      },
      spec: {
        allow: {
          [labelField]: labels,
        },
        deny: {},
        options: optionsWithDefaults(defaultRoleVersion),
      },
    };
  }

  test.each`
    labelField                  | labels
    ${'app_labels'}             | ${{ env: '{{internal.logins}}' }}
    ${'db_labels'}              | ${{ team: '{{external.groups}}' }}
    ${'kubernetes_labels'}      | ${{ namespace: '{{internal.kubernetes_users}}' }}
    ${'node_labels'}            | ${{ host: '{{internal.traits}}' }}
    ${'windows_desktop_labels'} | ${{ domain: '{{external.email}}' }}
    ${'app_labels'}             | ${{ env: ['{{internal.logins}}', 'prod'] }}
    ${'db_labels'}              | ${{ team: ['dev', '{{external.groups}}'] }}
  `(
    'returns unsupported-role-fields when $labelField contains interpolation',
    ({ labelField, labels }) => {
      const role = makeStandardRoleWithLabels(labelField, labels);
      const result = validateQueriedRoles({
        gotRoles: [role],
        accessList: makeAccessList({
          memberRolesGranted: [role.metadata.name],
        }),
      });

      expect(result.status).toBe('unsupported-role-fields');
    }
  );
});

describe('unknownFieldHasValue behavior', () => {
  function makeRoleWithUnknownField(unknownFieldValue: unknown): Role {
    return {
      kind: 'role',
      version: defaultRoleVersion,
      metadata: {
        name: `access-standard-acl-preset-${testAccessListId}`,
        labels: { [internalAccessListPresetLabelKey]: testAccessListId },
      },
      spec: {
        allow: {
          app_labels: { env: 'prod' },
          some_unknown_field: unknownFieldValue,
        } as Role['spec']['allow'],
        deny: {},
        options: optionsWithDefaults(defaultRoleVersion),
      },
    };
  }

  test.each`
    desc                  | unknownFieldValue
    ${'non-empty array'}  | ${[{ key: 'value' }]}
    ${'non-empty object'} | ${{ key: 'value' }}
    ${'non-empty string'} | ${'some-value'}
    ${'positive number'}  | ${42}
    ${'true boolean'}     | ${true}
  `(
    'returns unsupported-role-fields when unknown field has $desc',
    ({ unknownFieldValue }) => {
      const role = makeRoleWithUnknownField(unknownFieldValue);
      const result = validateQueriedRoles({
        gotRoles: [role],
        accessList: makeAccessList({
          memberRolesGranted: [role.metadata.name],
        }),
      });

      expect(result.status).toBe('unsupported-role-fields');
    }
  );

  test.each`
    desc               | unknownFieldValue
    ${'null'}          | ${null}
    ${'undefined'}     | ${undefined}
    ${'empty array'}   | ${[]}
    ${'empty object'}  | ${{}}
    ${'empty string'}  | ${''}
    ${'zero'}          | ${0}
    ${'false boolean'} | ${false}
  `(
    'returns valid-roles when unknown field has $desc (treated as empty)',
    ({ unknownFieldValue }) => {
      const role = makeRoleWithUnknownField(unknownFieldValue);
      const result = validateQueriedRoles({
        gotRoles: [role],
        accessList: makeAccessList({
          memberRolesGranted: [role.metadata.name],
        }),
      });

      expect(result.status).toBe('valid-roles');
    }
  );
});
