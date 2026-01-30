import {
  AccessListOrigin,
  AccessListType,
} from 'e-teleport/services/accessmanagement';

import { Action, getActionForbiddenInfo, isActionForbidden } from './access';

describe('isEditDisabled', () => {
  // 'true' means the action is forbidden while 'false' means it is allowed
  test.each`
    desc                                  | forOktaOrigin | forOktaRO | forOwner | forAdminWhoCanEdit | forAdminWhoCanDelete
    ${'EditMembers+undefined'}            | ${false}      | ${true}   | ${false} | ${false}           | ${true}
    ${'EditMembers+Default'}              | ${false}      | ${true}   | ${false} | ${false}           | ${true}
    ${'EditMembers+Scim'}                 | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditMembers+Static'}               | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditOwners+undefined'}             | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwners+Default'}               | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwners+Scim'}                  | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwners+Static'}                | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditTitleOrDescription+undefined'} | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditTitleOrDescription+Default'}   | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditTitleOrDescription+Scim'}      | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditTitleOrDescription+Static'}    | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditMembersEligibility+undefined'} | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersEligibility+Default'}   | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersEligibility+Scim'}      | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersEligibility+Static'}    | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditOwnersEligibility+undefined'}  | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwnersEligibility+Default'}    | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwnersEligibility+Scim'}       | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwnersEligibility+Static'}     | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditMembersGrants+undefined'}      | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersGrants+Default'}        | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersGrants+Scim'}           | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersGrants+Static'}         | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditOwnersGrants+undefined'}       | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwnersGrants+Default'}         | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwnersGrants+Scim'}            | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditOwnersGrants+Static'}          | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'EditAudit+undefined'}              | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditAudit+Default'}                | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditAudit+Scim'}                   | ${false}      | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditAudit+Static'}                 | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
    ${'Delete+undefined'}                 | ${false}      | ${true}   | ${true}  | ${true}            | ${false}
    ${'Delete+Default'}                   | ${false}      | ${true}   | ${true}  | ${true}            | ${false}
    ${'Delete+Scim'}                      | ${false}      | ${true}   | ${true}  | ${true}            | ${false}
    ${'Delete+Static'}                    | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
  `(
    `for $desc`,
    ({
      desc,
      forOktaOrigin,
      forOktaRO,
      forOwner,
      forAdminWhoCanEdit,
      forAdminWhoCanDelete,
    }) => {
      const action = valueOfAccessKind(desc.split('+')[0]);
      const accessListType = valueOfAccessListType(desc.split('+')[1]);
      const accessList = {
        type: accessListType,
        origin: undefined,
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
        },
        action: Action.EditMembersGrants,
        isReadOnlyOktaList: false,
        perms: adminPerms,
      };

      expect(isActionForbidden(props)).toBe(true);
      expect(getActionForbiddenInfo(props)).toBe(
        'Go to "Access Definition" tab to edit member access'
      );
    }
  );

  test('EditMembersGrants is allowed for access list without preset', () => {
    const props = {
      accessList: {
        type: AccessListType.Default,
        origin: undefined,
        preset: undefined,
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
        },
        action: Action.Delete,
        isReadOnlyOktaList: false,
        perms: adminPerms,
        missingRolePerms: ['role.read'],
      };

      expect(isActionForbidden(props)).toBe(true);
      expect(getActionForbiddenInfo(props)).toContain(
        'Insufficient permissions to delete this access list created with a guide'
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
