import {
  AccessListMember,
  AccessListMemberKind,
} from 'e-teleport/services/accessmanagement';

import {
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
} from './Shared';

describe('getEligibleUsersForAddingNewUsers', () => {
  // eslint-disable-next-line jest/require-hook
  [
    {
      case: 'empty',
      eligibility: { roles: [], traits: {} },
      fetchedUsers: [],
      existingUser: [],
      output: [],
    },
    {
      case: 'return empty if no eligibilty is defined',
      eligibility: { roles: [], traits: {} },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['admin'] }, label: '' },
      ],
      existingUser: [],
      output: [],
    },
    {
      case: 'eligible users with no existing users',
      eligibility: { roles: ['admin'], traits: {} },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['admin'] }, label: '' },
        { value: { name: 'test3', roles: ['access'] }, label: '' },
        { value: { name: 'test4', roles: ['admin'] }, label: '' },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
      ],
      existingUser: [],
      output: [
        {
          value: { name: 'test2', membershipKind: AccessListMemberKind.User },
          label: 'test2',
        },
        {
          value: { name: 'test4', membershipKind: AccessListMemberKind.User },
          label: 'test4',
        },
        {
          value: { name: 'test5', membershipKind: AccessListMemberKind.User },
          label: 'test5',
        },
      ],
    },
    {
      case: 'existing users extracted from eligible users',
      eligibility: { roles: ['admin'], traits: {} },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['admin'] }, label: '' },
        { value: { name: 'test3', roles: ['access'] }, label: '' },
        { value: { name: 'test4', roles: ['admin'] }, label: '' },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
      ],
      existingUser: [{ name: 'test2' }, { name: 'test5' }],
      output: [
        {
          value: { name: 'test4', membershipKind: AccessListMemberKind.User },
          label: 'test4',
        },
      ],
    },
    {
      case: 'no eligible users',
      eligibility: { roles: ['admin'], traits: {} },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['access'] }, label: '' },
        { value: { name: 'test3', roles: ['access'] }, label: '' },
        { value: { name: 'test4', roles: ['access'] }, label: '' },
        { value: { name: 'test5', roles: ['access'] }, label: '' },
      ],
      existingUser: [],
      output: [],
    },
    {
      case: 'multi eligibilty match',
      // users should at least have these roles to be eligible.
      eligibility: {
        roles: ['admin', 'access', 'editor'],
        traits: { fruit: ['apple'] },
      },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access', 'admin'] }, label: '' },
        { value: { name: 'test2', roles: ['admin', 'access'] }, label: '' },
        { value: { name: 'test3', roles: ['access', 'editor'] }, label: '' },
        {
          value: {
            name: 'test4',
            roles: ['editor', 'access', 'admin'],
            allTraits: { fruit: ['apple'] },
          },
          label: '',
        },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
        {
          value: {
            name: 'test6',
            roles: ['admin', 'access', 'editor'],
            allTraits: { fruit: ['apple'] },
          },
          label: '',
        },
        {
          value: {
            name: 'test7',
            roles: ['access', 'admin', 'access', 'editor'],
            allTraits: { fruit: ['apple'] },
          },
          label: '',
        },
        {
          value: { name: 'test8', roles: ['admin', 'access', 'intern'] },
          label: '',
        },
        {
          value: {
            name: 'test9',
            roles: ['access', 'admin', 'intern', 'access', 'editor'],
            allTraits: {
              fruit: ['banana', 'apple'],
              drink: ['water'],
            },
          },
          label: '',
        },
      ],
      existingUser: [],
      output: [
        {
          value: { name: 'test4', membershipKind: AccessListMemberKind.User },
          label: 'test4',
        },
        {
          value: { name: 'test6', membershipKind: AccessListMemberKind.User },
          label: 'test6',
        },
        {
          value: { name: 'test7', membershipKind: AccessListMemberKind.User },
          label: 'test7',
        },
        {
          value: { name: 'test9', membershipKind: AccessListMemberKind.User },
          label: 'test9',
        },
      ],
    },
  ].forEach(tc => {
    test(`case: ${tc.case}`, () => {
      const obj = getEligibleUsersForAddingNewUsers(
        tc.eligibility,
        tc.fetchedUsers,
        tc.existingUser
      );
      expect(obj).toStrictEqual(tc.output);
    });
  });
});

describe('getNewAndExistingUsersForAddingNewUsers', () => {
  // eslint-disable-next-line jest/require-hook
  [
    {
      case: 'empty lists',
      existingUsers: [],
      selectedUsers: [],
      output: {
        duplicateUsers: [],
        newUsers: [],
      },
    },
    {
      case: 'only selected users defined',
      existingUsers: [],
      selectedUsers: [
        { value: 'test', label: '' },
        { value: 'test1', label: '' },
      ],
      output: {
        duplicateUsers: [],
        newUsers: [
          {
            value: { name: 'test', membershipKind: AccessListMemberKind.User },
            label: '',
          },
          {
            value: { name: 'test1', membershipKind: AccessListMemberKind.User },
            label: '',
          },
        ],
      },
    },
    {
      case: 'only existing users defined',
      existingUsers: [{ name: 'test' }, { name: 'test1' }],
      selectedUsers: [],
      output: {
        duplicateUsers: [],
        newUsers: [],
      },
    },
    {
      case: 'existing users are extracted from selected',
      existingUsers: [{ name: 'test' }, { name: 'test3' }, { name: 'test4' }],
      selectedUsers: [
        { value: 'test', label: '' },
        { value: 'test1', label: '' },
        { value: 'test2', label: '' },
        { value: 'test3', label: '' },
        { value: 'test4', label: '' },
        { value: 'test5', label: '' },
      ],
      output: {
        duplicateUsers: ['test', 'test3', 'test4'],
        newUsers: [
          {
            value: { name: 'test1', membershipKind: AccessListMemberKind.User },
            label: '',
          },
          {
            value: { name: 'test2', membershipKind: AccessListMemberKind.User },
            label: '',
          },
          {
            value: { name: 'test5', membershipKind: AccessListMemberKind.User },
            label: '',
          },
        ],
      },
    },
    {
      case: 'only new users found',
      existingUsers: [{ name: 'test' }, { name: 'test1' }, { name: 'test2' }],
      selectedUsers: [
        { value: 'test3', label: '' },
        { value: 'test4', label: '' },
        { value: 'test5', label: '' },
      ],
      output: {
        duplicateUsers: [],
        newUsers: [
          {
            value: { name: 'test3', membershipKind: AccessListMemberKind.User },
            label: '',
          },
          {
            value: { name: 'test4', membershipKind: AccessListMemberKind.User },
            label: '',
          },
          {
            value: { name: 'test5', membershipKind: AccessListMemberKind.User },
            label: '',
          },
        ],
      },
    },
    {
      case: 'only duplicates found',
      existingUsers: [{ name: 'test' }, { name: 'test1' }, { name: 'test2' }],
      selectedUsers: [
        { value: 'test', label: '' },
        { value: 'test1', label: '' },
        { value: 'test2', label: '' },
      ],
      output: {
        duplicateUsers: ['test', 'test1', 'test2'],
        newUsers: [],
      },
    },
  ].forEach(tc => {
    test(`case: ${tc.case}`, () => {
      const obj = getNewAndExistingUsersForAddingNewUsers(
        tc.existingUsers as AccessListMember[],
        tc.selectedUsers.map(m => ({
          label: m.label,
          value: { name: m.value, membershipKind: AccessListMemberKind.User },
        }))
      );
      expect(obj).toStrictEqual(tc.output);
    });
  });
});
