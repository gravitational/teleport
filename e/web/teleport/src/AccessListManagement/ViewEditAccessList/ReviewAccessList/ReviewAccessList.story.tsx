import { addWeeks } from 'date-fns';
import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import {
  AccessListMemberKind,
  AccessListOrigin,
  AccessListType,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import cfg from 'teleport/config';

import { convertToTraitConvenience } from '../../Traits';
import type { AccessListModified } from '../Shared';
import { ReviewAccessList } from './ReviewAccessList';

export default {
  title: 'TeleportE/AccessLists/Review',
};

const getRolesHandler = http.get(
  cfg.getRoleUrl({ action: 'list' }),
  async () => {
    await delay(1000);
    return HttpResponse.json({
      items: [
        { id: 'id1', kind: 'role', name: 'access', content: '' },
        { id: 'id2', kind: 'role', name: 'admin', content: '' },
        { id: 'id3', kind: 'role', name: 'editor', content: '' },
        { id: 'id4', kind: 'role', name: 'foo', content: '' },
        { id: 'id5', kind: 'role', name: 'apple', content: '' },
        { id: 'id6', kind: 'role', name: 'banana', content: '' },
      ],
    });
  }
);

export const WithFullAccessList = () => {
  return (
    <MemoryRouter>
      <TeleportProviderBasicE>
        <Info>Devs: Click the buttons to see each step</Info>
        <ReviewAccessList
          cancelReview={() => null}
          accessList={mockAccessListFull}
          reviewer="llama"
          isOwner={false}
        />
      </TeleportProviderBasicE>
    </MemoryRouter>
  );
};
WithFullAccessList.parameters = {
  msw: {
    handlers: [getRolesHandler],
  },
};

export const WithFullScimAccessList = () => {
  return (
    <MemoryRouter>
      <TeleportProviderBasicE>
        <Info>Devs: Click the buttons to see each step</Info>
        <ReviewAccessList
          cancelReview={() => null}
          accessList={mockAccessListScim}
          reviewer="llama"
          isOwner={false}
        />
      </TeleportProviderBasicE>
    </MemoryRouter>
  );
};
WithFullScimAccessList.parameters = {
  msw: {
    handlers: [getRolesHandler],
  },
};

export const WithSparseAccessList = () => {
  return (
    <MemoryRouter>
      <TeleportProviderBasicE>
        <Info>Devs: Click the buttons to see each step</Info>
        <ReviewAccessList
          cancelReview={() => null}
          accessList={mockAccessListSparse}
          reviewer="llama"
          isOwner={false}
        />
      </TeleportProviderBasicE>
    </MemoryRouter>
  );
};
WithSparseAccessList.parameters = {
  msw: {
    handlers: [getRolesHandler],
  },
};

export const WithFullAccessListOwner = () => {
  return (
    <MemoryRouter>
      <TeleportProviderBasicE>
        <Info>Devs: Click the buttons to see each step</Info>
        <ReviewAccessList
          cancelReview={() => null}
          accessList={mockAccessListFull}
          reviewer="llama"
          isOwner={true}
        />
      </TeleportProviderBasicE>
    </MemoryRouter>
  );
};
WithFullAccessListOwner.parameters = {
  msw: {
    handlers: [getRolesHandler],
  },
};

export const WithSparseAccessListOwner = () => {
  return (
    <MemoryRouter>
      <TeleportProviderBasicE>
        <Info>Devs: Click the buttons to see each step</Info>
        <ReviewAccessList
          cancelReview={() => null}
          accessList={mockAccessListSparse}
          reviewer="llama"
          isOwner={true}
        />
      </TeleportProviderBasicE>
    </MemoryRouter>
  );
};
WithSparseAccessListOwner.parameters = {
  msw: {
    handlers: [getRolesHandler],
  },
};

const mockAccessListFull: AccessListModified = {
  id: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
  metadata: {
    name: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
    labels: {},
    revision: '',
  },
  type: AccessListType.Default,
  title: 'Interns',
  origin: AccessListOrigin.Okta,
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

const mockAccessListScim: AccessListModified = {
  ...mockAccessListFull,
  type: AccessListType.Scim,
  origin: AccessListOrigin.Unspecified,
};

const mockAccessListSparse: AccessListModified = {
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
