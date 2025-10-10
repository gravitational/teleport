import {
  AccessListMember,
  AccessListMemberKind,
} from 'e-teleport/services/accessmanagement';

import { getNewAndExistingUsersForAddingNewUsers } from './Shared';

describe('getNewAndExistingUsersForAddingNewUsers', () => {
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
