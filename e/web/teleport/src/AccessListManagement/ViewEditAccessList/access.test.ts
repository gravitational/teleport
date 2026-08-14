import {
  AccessListOrigin,
  AccessListType,
} from 'e-teleport/services/accessmanagement';

import { Action, getActionForbiddenInfo, isActionForbidden } from './access';

const emptyMetadata = { name: '', labels: {}, revision: '' };

describe('isEditDisabled', () => {
  // 'true' means the action is forbidden while 'false' means it is allowed
  test.each`
    desc                                  | forOktaOrigin | forOktaRO | forOwner | forAdminWhoCanEdit | forAdminWhoCanDelete | forOktaSyncEnabled | forOktaROSyncDisabled
    ${'EditMembers+undefined'}            | ${false}      | ${true}   | ${false} | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditMembers+Default'}              | ${false}      | ${true}   | ${false} | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditMembers+Scim'}                 | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditMembers+Static'}               | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditOwners+undefined'}             | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditOwners+Default'}               | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditOwners+Scim'}                  | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditOwners+Static'}                | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditTitleOrDescription+undefined'} | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditTitleOrDescription+Default'}   | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditTitleOrDescription+Scim'}      | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditTitleOrDescription+Static'}    | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditMembersEligibility+undefined'} | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditMembersEligibility+Default'}   | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditMembersEligibility+Scim'}      | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditMembersEligibility+Static'}    | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditOwnersEligibility+undefined'}  | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditOwnersEligibility+Default'}    | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditOwnersEligibility+Scim'}       | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditOwnersEligibility+Static'}     | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditMembersGrants+undefined'}      | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditMembersGrants+Default'}        | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditMembersGrants+Scim'}           | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditMembersGrants+Static'}         | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditOwnersGrants+undefined'}       | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditOwnersGrants+Default'}         | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditOwnersGrants+Scim'}            | ${true}       | ${true}   | ${true}  | ${false}           | ${true}              | ${true}            | ${true}
    ${'EditOwnersGrants+Static'}          | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'EditAudit+undefined'}              | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditAudit+Default'}                | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditAudit+Scim'}                   | ${false}      | ${true}   | ${true}  | ${false}           | ${true}              | ${false}           | ${true}
    ${'EditAudit+Static'}                 | ${true}       | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${true}
    ${'Delete+undefined'}                 | ${false}      | ${true}   | ${true}  | ${true}            | ${false}             | ${true}            | ${false}
    ${'Delete+Default'}                   | ${false}      | ${true}   | ${true}  | ${true}            | ${false}             | ${true}            | ${false}
    ${'Delete+Scim'}                      | ${false}      | ${true}   | ${true}  | ${true}            | ${false}             | ${true}            | ${false}
    ${'Delete+Static'}                    | ${false}      | ${true}   | ${true}  | ${true}            | ${true}              | ${true}            | ${false}
  `(
    `for $desc`,
    ({
      desc,
      forOktaOrigin,
      forOktaRO,
      forOwner,
      forAdminWhoCanEdit,
      forAdminWhoCanDelete,
      forOktaSyncEnabled,
      forOktaROSyncDisabled,
    }) => {
      const action = valueOfAccessKind(desc.split('+')[0]);
      const accessListType = valueOfAccessListType(desc.split('+')[1]);
      const accessList = {
        type: accessListType,
        origin: undefined,
        metadata: emptyMetadata,
      };
      const noPerms = {
        adminWhoCanRead: false,
        adminWhoCanDelete: false,
        adminWhoCanEdit: false,
        isOwner: false,
      };
      const fullPerms = {
        adminWhoCanRead: true,
        adminWhoCanDelete: true,
        adminWhoCanEdit: true,
        isOwner: true,
      };

      expect(
        isActionForbidden({
          accessList: { ...accessList, origin: AccessListOrigin.Okta },
          action: action,
          isReadOnlyOktaList: false,
          perms: fullPerms,
        })
      ).toBe(forOktaOrigin);

      expect(
        isActionForbidden({
          accessList: accessList,
          action: action,
          isReadOnlyOktaList: true,
          perms: fullPerms,
        })
      ).toBe(forOktaRO);

      expect(
        isActionForbidden({
          accessList: accessList,
          action: action,
          isReadOnlyOktaList: false,
          perms: { ...noPerms, isOwner: true },
        })
      ).toBe(forOwner);

      expect(
        isActionForbidden({
          accessList: accessList,
          action: action,
          isReadOnlyOktaList: false,
          perms: { ...noPerms, adminWhoCanEdit: true },
        })
      ).toBe(forAdminWhoCanEdit);

      expect(
        isActionForbidden({
          accessList: accessList,
          action: action,
          isReadOnlyOktaList: false,
          perms: { ...noPerms, adminWhoCanDelete: true },
        })
      ).toBe(forAdminWhoCanDelete);

      expect(
        isActionForbidden({
          accessList: { ...accessList, origin: AccessListOrigin.Okta },
          action,
          isOktaEnableAccessListSync: true,
          perms: fullPerms,
        })
      ).toBe(forOktaSyncEnabled);

      expect(
        isActionForbidden({
          accessList: { ...accessList, origin: AccessListOrigin.Okta },
          action,
          isReadOnlyOktaList: true,
          isOktaEnableAccessListSync: false,
          perms: fullPerms,
        })
      ).toBe(forOktaROSyncDisabled);
    }
  );
});

function valueOfAccessKind(key: string): Action {
  if (!Object.keys(Action).includes(key))
    throw new Error(`'${key}' is not a member of AccessKind`);
  return Action[key];
}

function valueOfAccessListType(key: string): AccessListType {
  if (key === 'undefined') return undefined;
  if (!Object.keys(AccessListType).includes(key))
    throw new Error(`'${key}' is not a member of AccessListType`);
  return AccessListType[key];
}

describe('access list with preset', () => {
  const adminPerms = {
    adminWhoCanRead: true,
    adminWhoCanDelete: true,
    adminWhoCanEdit: true,
    isOwner: false,
  };

  test.each(['long-term', 'short-term'] as const)(
    'EditMembersGrants is forbidden for %s preset',
    preset => {
      const props = {
        accessList: {
          type: AccessListType.Default,
          origin: undefined,
          preset,
          metadata: emptyMetadata,
        },
        action: Action.EditMembersGrants,
        isReadOnlyOktaList: false,
        perms: adminPerms,
      };

      expect(isActionForbidden(props)).toBe(true);
      expect(getActionForbiddenInfo(props)).toBe(
        'Go to "Access Definition" tab to edit members access to resources.'
      );
    }
  );

  test('EditMembersGrants is allowed for access list without preset', () => {
    const props = {
      accessList: {
        type: AccessListType.Default,
        origin: undefined,
        preset: undefined,
        metadata: emptyMetadata,
      },
      action: Action.EditMembersGrants,
      isReadOnlyOktaList: false,
      perms: adminPerms,
    };

    expect(isActionForbidden(props)).toBe(false);
    expect(getActionForbiddenInfo(props)).toBeUndefined();
  });

  test.each(['long-term', 'short-term'] as const)(
    'Delete is forbidden for %s preset if missing role perms',
    preset => {
      const props = {
        accessList: {
          type: AccessListType.Default,
          origin: undefined,
          preset,
          metadata: emptyMetadata,
        },
        action: Action.Delete,
        isReadOnlyOktaList: false,
        perms: adminPerms,
        missingRolePerms: ['role.read'],
      };

      expect(isActionForbidden(props)).toBe(true);
      expect(getActionForbiddenInfo(props)).toContain(
        'Unable to delete this access list'
      );
    }
  );

  test.each(['long-term', 'short-term'] as const)(
    'Delete is allowed for %s preset if no missing role perms',
    preset => {
      const props = {
        accessList: {
          type: AccessListType.Default,
          origin: undefined,
          preset,
          metadata: emptyMetadata,
        },
        action: Action.Delete,
        isReadOnlyOktaList: false,
        perms: adminPerms,
        missingRolePerms: undefined,
      };

      expect(isActionForbidden(props)).toBe(false);
      expect(getActionForbiddenInfo(props)).toBeUndefined();
    }
  );

  test('Delete is allowed when no preset is used', () => {
    const props = {
      accessList: {
        type: AccessListType.Default,
        origin: undefined,
        preset: undefined,
        metadata: emptyMetadata,
      },
      action: Action.Delete,
      isReadOnlyOktaList: false,
      perms: adminPerms,
      missingRolePerms: ['role.read'], // no affect
    };

    expect(isActionForbidden(props)).toBe(false);
    expect(getActionForbiddenInfo(props)).toBeUndefined();
  });
});

describe('EntraID Access Control', () => {
  const fullPerms = {
    adminWhoCanRead: true,
    adminWhoCanDelete: true,
    adminWhoCanEdit: true,
    isOwner: true,
  };

  test('EditMembers is forbidden for EntraID origin', () => {
    const props = {
      accessList: {
        type: AccessListType.Default,
        origin: AccessListOrigin.EntraID,
        metadata: emptyMetadata,
      },
      action: Action.EditMembers,
      isReadOnlyOktaList: false,
      perms: fullPerms,
    };

    expect(isActionForbidden(props)).toBe(true);
    expect(getActionForbiddenInfo(props)).toBe(
      'Editing members is disabled; this Access List is managed by Entra ID'
    );
  });

  test('Other actions are allowed for EntraID origin (if perms allow)', () => {
    const actions = [
      Action.EditOwners,
      Action.EditTitleOrDescription,
      Action.EditAudit,
      Action.Delete,
    ];

    actions.forEach(action => {
      const props = {
        accessList: {
          type: AccessListType.Default,
          origin: AccessListOrigin.EntraID,
          metadata: emptyMetadata,
        },
        action,
        isReadOnlyOktaList: false,
        perms: fullPerms,
      };

      expect(isActionForbidden(props)).toBe(false);
      expect(getActionForbiddenInfo(props)).toBeUndefined();
    });
  });
});
