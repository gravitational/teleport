import { addWeeks } from 'date-fns';

import {
  AccessListMemberKind,
  AccessListType,
  IneligibleStatus,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';

export const rawAccessList = {
  metadata: {
    name: 'mock-access-list-id',
  },
  user_displays: {
    member1: {
      primary: 'Member One With A Long Display Name For Truncation',
      secondary: 'Infrastructure Engineering With A Long Team Name',
    },
    'lisa@goteleport.com': {
      primary: 'Lisa Access List Owner',
      secondary: 'Identity Governance',
    },
    owner1: {
      primary: 'Owner One With A Long Display Name For Truncation',
      secondary: 'Security Engineering With A Long Team Name',
    },
  },
  members: [
    {
      name: 'member1',
      joined: '2023-05-24T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      reason: 'some reason',
      added_by: 'lisa@goteleport.com',
      ineligible_status: IneligibleStatus.Expired,
      membership_kind: AccessListMemberKind.User,
    },
    {
      name: 'mock-nested-access-list-id',
      joined: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
      expires: '',
      added_by: 'maxim@goteleport.com',
      membership_kind: AccessListMemberKind.List,
    },
    {
      name: 'member2',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      added_by: 'llama',
      membership_kind: AccessListMemberKind.User,
    },
    {
      name: 'member3',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: '2024-12-12T17:48:15.78579Z',
      added_by: 'alpaca',
      membership_kind: AccessListMemberKind.User,
    },
  ],
  // this is a bit of a hack, since we're using the same obj for the list resp and the single resp.
  inherited_member_grants: {
    roles: [],
    traits: {},
  },
  spec: {
    title: 'Mock Access List Title',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua',
    owners: [
      {
        name: 'owner1',
        description: 'some description',
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'george.washington@goteleport.com',
        ineligible_status: IneligibleStatus.MissingRequirements,
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'llama',
        membership_kind: AccessListMemberKind.User,
      },
    ],
    grants: {
      roles: ['access', 'editor'],
      traits: { fruit: ['apple'] },
      scoped_roles: [{ role: '/::team-admin', scope: '/dev/platform' }],
    },
    owner_grants: {
      roles: ['admin', 'almighty'],
      traits: { status: ['pro'] },
      scoped_roles: [{ role: 'audit', scope: '/**' }],
    },
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneMonth,
        dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
      },
      next_audit_date: new Date().toString(),
    },
    ownership_requires: {
      roles: ['admin'],
      traits: { fruit: ['banana', 'apple'], drink: ['coffee'] },
    },
    membership_requires: {
      roles: ['reviewer', 'auditor'],
      traits: { fruit: ['carrot'] },
    },
  },
};

export const rawEmptyAccessList = {
  metadata: {
    name: 'mock-access-list-id',
  },
  spec: {
    title: 'Example of an empty Access List',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneMonth,
        dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
      },
      next_audit_date: addWeeks(new Date(), 8).toString(),
    },
  },
};

export const rawAccessListAsOwner = {
  ...rawAccessList,
  current_user_assignments: {
    ownership_type: 1,
  },
};

export const rawAccessListAsMember = {
  ...rawAccessList,
  current_user_assignments: {
    membership_type: 1,
  },
};

export const rawAccessListOkta = {
  ...rawAccessList,
  metadata: {
    ...rawAccessList.metadata,
    labels: {
      'okta/org': 'https://some-url',
    },
  },
};

export const rawAccessListStatic = {
  ...rawAccessList,
  spec: {
    ...rawAccessList.spec,
    title: 'Mock Static Access List Title',
    type: AccessListType.Static,
  },
};

export const rawAccessListScim = {
  ...rawAccessList,
  spec: {
    ...rawAccessList.spec,
    title: 'Mock Scim Access List Title',
    type: AccessListType.Scim,
  },
};

export const rawAccessListEntraID = {
  ...rawAccessList,
  metadata: {
    ...rawAccessList.metadata,
    labels: {
      'teleport.dev/origin': 'entra-id',
    },
  },
  spec: {
    ...rawAccessList.spec,
    title: 'Mock Entra ID Access List Title',
  },
};

export const rawNestedAccessList = {
  metadata: {
    name: 'mock-nested-access-list-id',
    labels: {},
  },
  members: [
    {
      name: 'member1',
      joined: '2023-05-24T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      reason: 'some reason',
      added_by: 'maxim@goteleport.com',
      ineligible_status: IneligibleStatus.Expired,
      membership_kind: AccessListMemberKind.User,
    },
    {
      name: 'member2',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
      added_by: 'maxim@goteleport.com',
      ineligible_status: undefined,
      membership_kind: AccessListMemberKind.User,
    },
  ],
  inherited_member_grants: rawAccessList.spec.grants,
  spec: {
    title: 'Mock Nested Access List',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua',
    owners: [
      {
        name: 'owner1',
        description: 'some description',
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'llama',
        membership_kind: AccessListMemberKind.User,
      },
    ],
    grants: {
      roles: ['access'],
      traits: { fruit: ['orange'] },
    },
    owner_grants: {
      roles: [],
      traits: {},
    },
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
      },
      next_audit_date: new Date(
        Date.now() + 365 * 24 * 60 * 60 * 1000
      ).toISOString(),
    },
    ownership_requires: {
      roles: [],
      traits: {},
    },
    membership_requires: {
      roles: [],
      traits: {},
    },
  },
};

export const rawReviewsResponse = {
  reviews: [
    {
      kind: 'access_list_review',
      version: 'v1',
      metadata: {
        name: '147879cd-5d78-4c10-a215-c6339dfada72',
        expires: '0001-01-01T00:00:00Z',
        revision: '8ec832a4-6960-479a-9230',
      },
      spec: {
        review_date: '2025-06-26T23:09:50.604529Z',
        access_list: '270a3348-3711-421e-a053-6914da6aeb46',
        reviewers: ['lisa'],
        notes: '',
        changes: {
          review_frequency_changed: '',
          review_day_of_month_changed: '',
          membership_requirements_changed: null,
          removed_members: null,
        },
      },
      reviewersInfo: [
        {
          username: 'lisa',
          display: {
            primary: 'Lisa Reviewer With A Long Display Name For Truncation',
            secondary: 'Identity Governance',
          },
        },
      ],
    },
    {
      kind: 'access_list_review',
      version: 'v1',
      metadata: {
        name: 'ddd24f19-22e7-46ea-a539-b6a0a1a84b39',
        expires: '0001-01-01T00:00:00Z',
        revision: '4aeac4af-79eb-41fa-a089-56c322e5a13a',
      },
      spec: {
        review_date: '2025-06-30T22:21:16.684256Z',
        access_list: '270a3348-3711-421e-a053-6914da6aeb46',
        reviewers: ['lisa', 'another-one'],
        notes: 'some kind of note',
        changes: {
          review_frequency_changed: '1 month',
          review_day_of_month_changed: 'last',
          membership_requirements_changed: {
            roles: ['apple'],
            traits: null,
          },
          removed_members: ['llama', 'alpaca'],
        },
      },
      reviewersInfo: [
        {
          username: 'lisa',
          display: {
            primary: 'Lisa Reviewer With A Long Display Name For Truncation',
            secondary: 'Identity Governance',
          },
        },
        { username: 'another-one' },
      ],
    },
  ],
  startKey: '',
};
