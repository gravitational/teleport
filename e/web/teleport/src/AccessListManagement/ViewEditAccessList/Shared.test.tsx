import { render, screen } from 'design/utils/testing';

import {
  AccessListMember,
  AccessListMemberKind,
} from 'e-teleport/services/accessmanagement';

import {
  AlreadyEnrolledUsersAlert,
  getNewAndExistingUsersForAddingNewUsers,
} from './Shared';

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
      existingUsers: [
        { name: 'test', displayPrimary: 'Test User' },
        { name: 'test3' },
        { name: 'test4' },
      ],
      selectedUsers: [
        { value: 'test', label: '' },
        { value: 'test1', label: '' },
        { value: 'test2', label: '' },
        { value: 'test3', label: '' },
        { value: 'test4', label: '' },
        { value: 'test5', label: '' },
      ],
      output: {
        duplicateUsers: [
          { name: 'test', displayPrimary: 'Test User' },
          { name: 'test3' },
          { name: 'test4' },
        ],
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
        duplicateUsers: [
          { name: 'test' },
          { name: 'test1' },
          { name: 'test2' },
        ],
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

test('already enrolled alert renders displays without secondary text', () => {
  const users = [
    {
      name: '100002',
      displayPrimary: 'Marc Dubois',
      displaySecondary: 'marc@example.com',
    },
    {
      name: 'username-only',
    },
  ];

  render(<AlreadyEnrolledUsersAlert users={users} />);

  expect(
    screen.getByText(/The following users are already enrolled/)
  ).toBeVisible();
  expect(screen.getByText('Marc Dubois')).toBeVisible();
  expect(screen.getByText('100002')).toBeVisible();
  expect(screen.getByText('username-only')).toBeVisible();
  expect(screen.queryByText('marc@example.com')).not.toBeInTheDocument();
});
