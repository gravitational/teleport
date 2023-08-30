import {
  getEligibleUsersForAddingNewUsers,
  getNewAndExistingUsersForAddingNewUsers,
} from './Shared';

describe('getEligibleUsersForAddingNewUsers', () => {
  // eslint-disable-next-line jest/require-hook
  [
    {
      case: 'empty lists',
      eligibility: { roles: [] },
      fetchedUsers: [],
      existingUser: [],
      output: [],
    },
    {
      case: 'only eligibility defined, should return empty',
      eligibility: { roles: ['admin'] },
      fetchedUsers: [],
      existingUser: [],
      output: [],
    },
    {
      case: 'return all fetched users if no eligibility and existing users defined',
      eligibility: { roles: [] },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['admin'] }, label: '' },
        { value: { name: 'test3', roles: ['access'] }, label: '' },
        { value: { name: 'test4', roles: ['admin'] }, label: '' },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
      ],
      existingUser: [],
      output: [
        { value: 'test', label: 'test' },
        { value: 'test2', label: 'test2' },
        { value: 'test3', label: 'test3' },
        { value: 'test4', label: 'test4' },
        { value: 'test5', label: 'test5' },
      ],
    },
    {
      case: 'eligible users with no existing users',
      eligibility: { roles: ['admin'] },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['admin'] }, label: '' },
        { value: { name: 'test3', roles: ['access'] }, label: '' },
        { value: { name: 'test4', roles: ['admin'] }, label: '' },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
      ],
      existingUser: [],
      output: [
        { value: 'test2', label: 'test2' },
        { value: 'test4', label: 'test4' },
        { value: 'test5', label: 'test5' },
      ],
    },
    {
      case: 'existing users extracted from eligible users',
      eligibility: { roles: ['admin'] },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access'] }, label: '' },
        { value: { name: 'test2', roles: ['admin'] }, label: '' },
        { value: { name: 'test3', roles: ['access'] }, label: '' },
        { value: { name: 'test4', roles: ['admin'] }, label: '' },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
      ],
      existingUser: [{ name: 'test2' }, { name: 'test5' }],
      output: [{ value: 'test4', label: 'test4' }],
    },
    {
      case: 'no eligible users',
      eligibility: { roles: ['admin'] },
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
      case: 'multi eligibilty match and multi roles',
      // users should at least have these roles to be eligible.
      eligibility: { roles: ['admin', 'access', 'editor'] },
      fetchedUsers: [
        { value: { name: 'test', roles: ['access', 'admin'] }, label: '' },
        { value: { name: 'test2', roles: ['admin', 'access'] }, label: '' },
        { value: { name: 'test3', roles: ['access', 'editor'] }, label: '' },
        {
          value: { name: 'test4', roles: ['editor', 'access', 'admin'] },
          label: '',
        },
        { value: { name: 'test5', roles: ['admin'] }, label: '' },
        {
          value: { name: 'test6', roles: ['admin', 'access', 'editor'] },
          label: '',
        },
        {
          value: {
            name: 'test7',
            roles: ['access', 'admin', 'access', 'editor'],
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
          },
          label: '',
        },
      ],
      existingUser: [],
      output: [
        { value: 'test4', label: 'test4' },
        { value: 'test6', label: 'test6' },
        { value: 'test7', label: 'test7' },
        { value: 'test9', label: 'test9' },
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
          { value: 'test', label: '' },
          { value: 'test1', label: '' },
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
          { value: 'test1', label: '' },
          { value: 'test2', label: '' },
          { value: 'test5', label: '' },
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
          { value: 'test3', label: '' },
          { value: 'test4', label: '' },
          { value: 'test5', label: '' },
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
        tc.existingUsers,
        tc.selectedUsers
      );
      expect(obj).toStrictEqual(tc.output);
    });
  });
});
