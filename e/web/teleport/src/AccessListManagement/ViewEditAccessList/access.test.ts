import {
  AccessListOrigin,
  AccessListType,
} from 'e-teleport/services/accessmanagement';

import { Action, isActionForbidden } from './access';

describe('isEditDisabled', () => {
  test.each`
    desc                                  | forOktaOrigin | forOktaRO | forOwner | forAdminWhoCanEdit | forAdminWhoCanDelete
    ${'EditMembers+undefined'}            | ${false}      | ${true}   | ${false} | ${false}           | ${true}
    ${'EditMembers+Default'}              | ${false}      | ${true}   | ${false} | ${false}           | ${true}
    ${'EditMembers+Scim'}                 | ${false}      | ${true}   | ${false} | ${false}           | ${true}
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
    ${'EditMembersGrants+undefined'}      | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersGrants+Default'}        | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersGrants+Scim'}           | ${true}       | ${true}   | ${true}  | ${false}           | ${true}
    ${'EditMembersGrants+Static'}         | ${true}       | ${true}   | ${true}  | ${true}            | ${true}
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
