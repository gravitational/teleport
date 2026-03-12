import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';
import { convertToTraitConvenience } from 'e-teleport/AccessListManagement/Traits';
import {
  AccessListMember,
  AccessListMemberKind,
  AccessListOrigin,
  AccessListType,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';

import { AccessListModified } from '../Shared';
import { getEditedAccessListFields } from './ReviewAccessList';
import { ReviewMembers } from './ReviewMembers';

test('getEditedAccessListFields: no edits', () => {
  const req = getEditedAccessListFields({
    accessList: mockAccessList,
    editedMembers: mockAccessList.members,
    editedMembershipRequires: mockAccessList.membershipRequires,
    editedRecurrence: {
      reviewDayOfMonth: getReviewDayOfMonthOption(
        mockAccessList.audit.recurrence.dayOfMonth
      ),
      reviewFrequency: getReviewFrequencyOption(
        mockAccessList.audit.recurrence.frequency
      ),
    },
  });

  expect(req).toStrictEqual({
    membersDeleted: null,
    membershipRequires: null,
    auditRecurrence: {
      frequency: null,
      dayOfMonth: null,
    },
  });
});

test('getEditedAccessListFields: all fields edited', () => {
  const req = getEditedAccessListFields({
    accessList: mockAccessList,
    editedMembers: keepMembers, // deleted donkey and shrek
    editedMembershipRequires: {
      roles: ['access'], // deleted editor from mock
      ...convertToTraitConvenience({ holiday: ['halloween'] }), // changed from christmas
    },
    editedRecurrence: {
      // changed from 15th day
      reviewDayOfMonth: getReviewDayOfMonthOption(
        ReviewDayOfMonth.LastDayOfMonth
      ),
      // changed from one year
      reviewFrequency: getReviewFrequencyOption(ReviewFrequency.ThreeMonths),
    },
  });

  expect(req).toStrictEqual({
    membersDeleted: deleteMembers,
    membershipRequires: {
      roles: ['access'],
      traits: { holiday: ['halloween'] },
    },
    auditRecurrence: {
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      frequency: ReviewFrequency.ThreeMonths,
    },
  });
});

test('getEditedAccessListFields: one of each field edited: membership traits and audit frequency', () => {
  const req = getEditedAccessListFields({
    accessList: mockAccessList,
    editedMembers: mockAccessList.members,
    editedMembershipRequires: {
      roles: mockAccessList.membershipRequires.roles,
      ...convertToTraitConvenience({ holiday: ['halloween'] }), // changed from christmas
    },
    editedRecurrence: {
      reviewDayOfMonth: getReviewDayOfMonthOption(
        mockAccessList.audit.recurrence.dayOfMonth
      ),
      // changed from one year
      reviewFrequency: getReviewFrequencyOption(ReviewFrequency.OneMonth),
    },
  });

  expect(req).toStrictEqual({
    membersDeleted: null,
    membershipRequires: {
      roles: mockAccessList.membershipRequires.roles,
      traits: { holiday: ['halloween'] },
    },
    auditRecurrence: {
      dayOfMonth: null,
      frequency: ReviewFrequency.OneMonth,
    },
  });
});

test('getEditedAccessListFields: one of each field edited: membership roles and audit day of month', () => {
  const req = getEditedAccessListFields({
    accessList: mockAccessList,
    editedMembers: mockAccessList.members,
    editedMembershipRequires: {
      roles: ['access'], // deleted editor from mock
      traitLabels: mockAccessList.membershipRequires.traitLabels,
    },
    editedRecurrence: {
      // changed from 15th day
      reviewDayOfMonth: getReviewDayOfMonthOption(
        ReviewDayOfMonth.LastDayOfMonth
      ),
      reviewFrequency: getReviewFrequencyOption(
        mockAccessList.audit.recurrence.frequency
      ),
    },
  });

  expect(req).toStrictEqual({
    membersDeleted: null,
    membershipRequires: {
      roles: ['access'],
      traits: { holiday: ['christmas'] },
    },
    auditRecurrence: {
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      frequency: null,
    },
  });
});

test('getEditedAccessListFields: members with differing references are not flagged as deleted', () => {
  const req = getEditedAccessListFields({
    accessList: {
      ...mockAccessList,
      // clone members with the same data but new object references
      members: mockAccessList.members.map(m => ({
        ...m,
      })),
    },
    editedMembers: keepMembers,
    editedMembershipRequires: mockAccessList.membershipRequires,
    editedRecurrence: {
      reviewDayOfMonth: getReviewDayOfMonthOption(
        mockAccessList.audit.recurrence.dayOfMonth
      ),
      reviewFrequency: getReviewFrequencyOption(
        mockAccessList.audit.recurrence.frequency
      ),
    },
  });

  // donkey and shrek should be flagged as deleted
  expect(req.membersDeleted).toEqual(deleteMembers);
});

describe('ReviewMembers UI', () => {
  test('Remove buttons should be disabled for EntraID access lists', () => {
    const mockEntraIDAccessList: AccessListModified = {
      ...mockAccessList,
      origin: AccessListOrigin.EntraID,
    };

    render(
      <MemoryRouter>
        <ReviewMembers
          accessList={mockEntraIDAccessList}
          editedMembers={mockEntraIDAccessList.members}
          originalMembers={mockEntraIDAccessList.members}
          onDeleteMember={jest.fn()}
        />
      </MemoryRouter>
    );

    const removeButtons = screen.getAllByRole('button', { name: /remove/i });
    removeButtons.forEach(button => {
      expect(button).toBeDisabled();
    });

    // Also verify the alert message is present
    expect(
      screen.getByText(/Editing members is disabled/i)
    ).toBeInTheDocument();
  });
});

const deleteMembers = [
  {
    name: 'donkey',
    title: 'donkey',
    joined: new Date(),
    addedBy: 'some-owner',
    membershipKind: AccessListMemberKind.User,
  },
  {
    name: 'shrek',
    title: 'shrek',
    joined: new Date(),
    addedBy: 'fiona',
    membershipKind: AccessListMemberKind.User,
  },
] satisfies AccessListMember[];

const keepMembers = [
  {
    name: 'alpaca',
    title: 'alpaca',
    joined: new Date(),
    addedBy: 'lisa',
    ineligibleReason: 'should not show up',
    membershipKind: AccessListMemberKind.User,
  },
  {
    name: 'llama',
    title: 'llama',
    joined: new Date(),
    addedBy: 'lisa',
    ineligibleReason: 'should not show up',
    membershipKind: AccessListMemberKind.User,
  },
] satisfies AccessListMember[];

const mockAccessList: AccessListModified = {
  id: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
  metadata: {
    name: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
    labels: {},
    revision: '',
  },
  type: AccessListType.Default,
  title: 'Interns',
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date(),
  },
  grants: {
    roles: ['foo', 'bar', 'baz'],
    traits: { os: ['window'] },
    ...convertToTraitConvenience({ os: ['window'] }),
  },
  ownerGrants: {
    roles: ['admin'],
    traits: { status: ['root'] },
    ...convertToTraitConvenience({ status: ['root'] }),
  },
  ownershipRequires: {
    roles: ['admin'],
    traits: { power: ['admin-trait'] },
    ...convertToTraitConvenience({ power: ['admin-trait'] }),
  },
  membershipRequires: {
    roles: ['access', 'editor'],
    traits: { holiday: ['christmas'] },
    ...convertToTraitConvenience({ holiday: ['christmas'] }),
  },
  owners: [
    {
      name: 'some-owner',
      title: 'some-owner',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  members: [...keepMembers, ...deleteMembers],
  requiresReview: true,
  inheritedMemberGrants: { roles: [], traits: {} },
};
