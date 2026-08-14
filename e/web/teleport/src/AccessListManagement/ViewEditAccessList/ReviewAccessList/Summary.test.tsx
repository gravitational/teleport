import { getMemberApprovedMsg } from './Summary';

describe('getMemberApprovedMsg combo', () => {
  [
    {
      case: 'all 0 values',
      req: {
        numMembersApproved: 0,
        numRolesApproved: 0,
        numTraitsApproved: 0,
      },
      output: `0 members approved`,
    },
    {
      case: 'with members',
      req: {
        numMembersApproved: 5,
        numRolesApproved: 0,
        numTraitsApproved: 0,
      },
      output: `5 members approved`,
    },
    {
      case: 'with members and roles',
      req: {
        numMembersApproved: 5,
        numRolesApproved: 2,
        numTraitsApproved: 0,
      },
      output: `5 members approved to access 2 roles`,
    },
    {
      case: 'with members and roles and traits',
      req: {
        numMembersApproved: 5,
        numRolesApproved: 2,
        numTraitsApproved: 3,
      },
      output: `5 members approved to access 2 roles and 3 traits`,
    },
    {
      case: 'all singular items',
      req: {
        numMembersApproved: 1,
        numRolesApproved: 1,
        numTraitsApproved: 1,
      },
      output: `1 member approved to access 1 role and 1 trait`,
    },
  ].forEach(tc => {
    test(`case: ${tc.case}`, () => {
      const msg = getMemberApprovedMsg(tc.req);
      expect(msg).toStrictEqual(tc.output);
    });
  });
});
