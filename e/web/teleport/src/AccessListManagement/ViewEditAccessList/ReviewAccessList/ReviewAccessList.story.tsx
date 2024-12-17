import { addWeeks } from 'date-fns';
import { Info } from 'design/Alert';
import { MemoryRouter } from 'react-router';
import { Option } from 'shared/components/Select';

import {
  AccessListMemberKind,
  ReviewDayOfMonth,
  ReviewFrequency,
  AccessListType,
} from 'e-teleport/services/accessmanagement';

import { convertToTraitConvenience } from '../../Traits';

import { ReviewAccessList } from './ReviewAccessList';

import type { AccessListModified } from '../Shared';

export default {
  title: 'TeleportE/AccessLists/Review',
};

async function filterMockRoleOptions(input: string) {
  return mockRoleOptions.filter(r => r.value.includes(input));
}

export const WithFullAccessList = () => {
  return (
    <MemoryRouter>
      <Info>Devs: Click the buttons to see each step</Info>
      <ReviewAccessList
        cancelReview={() => null}
        accessList={mockAccessListFull}
        fetchRoleOptions={filterMockRoleOptions}
        reviewer="llama"
        isOwner={false}
      />
    </MemoryRouter>
  );
};

export const WithSparseAccessList = () => {
  return (
    <MemoryRouter>
      <Info>Devs: Click the buttons to see each step</Info>
      <ReviewAccessList
        cancelReview={() => null}
        accessList={mockAccessListSparse}
        fetchRoleOptions={filterMockRoleOptions}
        reviewer="llama"
        isOwner={false}
      />
    </MemoryRouter>
  );
};

export const WithFullAccessListOwner = () => {
  return (
    <MemoryRouter>
      <Info>Devs: Click the buttons to see each step</Info>
      <ReviewAccessList
        cancelReview={() => null}
        accessList={mockAccessListFull}
        fetchRoleOptions={filterMockRoleOptions}
        reviewer="llama"
        isOwner={true}
      />
    </MemoryRouter>
  );
};

export const WithSparseAccessListOwner = () => {
  return (
    <MemoryRouter>
      <Info>Devs: Click the buttons to see each step</Info>
      <ReviewAccessList
        cancelReview={() => null}
        accessList={mockAccessListSparse}
        fetchRoleOptions={filterMockRoleOptions}
        reviewer="llama"
        isOwner={true}
      />
    </MemoryRouter>
  );
};

const mockAccessListFull: AccessListModified = {
  id: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
  title: 'Interns',
  type: AccessListType.Okta,
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: addWeeks(new Date(), 1),
  },
  grants: {
    roles: ['foo', 'bar', 'baz'],
    traits: { os: ['window'] },
    ...convertToTraitConvenience({ os: ['window'] }),
  },
  ownerGrants: {
    roles: ['admin', 'root'],
    traits: { os: ['mac'] },
    ...convertToTraitConvenience({ os: ['mac'] }),
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
      title: 'owner',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  members: [
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
  ],
  requiresReview: true,
  inheritedMemberGrants: { roles: [], traits: {} },
};

const mockAccessListSparse: AccessListModified = {
  id: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
  title: 'Interns',
  audit: {
    recurrence: {
      frequency: ReviewFrequency.SixMonths,
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
    },
    nextDate: addWeeks(new Date(), 1),
  },
  grants: { roles: ['access'], traits: {}, traitLabels: [], traitList: [] },
  ownerGrants: { roles: ['admin'], traits: {}, traitLabels: [], traitList: [] },
  ownershipRequires: { roles: [], traits: {}, traitLabels: [], traitList: [] },
  owners: [],
  members: [],
  membershipRequires: {
    roles: [],
    traits: {},
    traitLabels: [],
    traitList: [],
  },
  requiresReview: true,
  inheritedMemberGrants: { roles: [], traits: {} },
};

const mockRoleOptions: Option[] = [
  { value: 'access', label: 'access' },
  { value: 'admin', label: 'admin' },
  { value: 'editor', label: 'editor' },
  { value: 'foo', label: 'foo' },
  { value: 'bar', label: 'bar' },
  { value: 'baz', label: 'baz' },
  { value: 'apple', label: 'apple' },
  { value: 'banana', label: 'banana' },
];
