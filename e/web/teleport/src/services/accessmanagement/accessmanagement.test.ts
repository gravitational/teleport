import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import {
  accessManagementService,
  convertReviewFrequencyIntoBackendParsableValue,
  makeAccessList,
} from './accessmanagement';
import {
  AccessList,
  AccessListMemberKind,
  AccessListOrigin,
  AccessListType,
  IneligibleStatus,
  ReviewDayOfMonth,
  ReviewFrequency,
  UpsertAccessListRequest,
} from './types';

test('fetch access lists, empty responses does not throw error', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ accessLists: null });
  let response = await accessManagementService.fetchAccessLists();
  expect(response).toStrictEqual([]);

  jest.spyOn(api, 'get').mockResolvedValue({ accessLists: [{}] });

  response = await accessManagementService.fetchAccessLists();
  expect(response).toStrictEqual([
    {
      id: '',
      metadata: {},
      type: AccessListType.Default,
      origin: '',
      preset: '',
      title: '',
      description: '',
      owners: [],
      members: [],
      membersCount: undefined,
      memberListCount: undefined,
      grants: {
        roles: [],
        traits: {},
        scopedRoles: [],
      },
      ownerGrants: {
        roles: [],
        traits: {},
        scopedRoles: [],
      },
      inheritedMemberGrants: {
        roles: [],
        traits: {},
        scopedRoles: [],
      },
      audit: {
        recurrence: {
          dayOfMonth: '',
          frequency: '',
        },
        nextDate: undefined,
      },
      currentUserAssignments: undefined,
      userAssignments: undefined,
      ownershipRequires: {
        roles: [],
        traits: {},
      },
      membershipRequires: {
        roles: [],
        traits: {},
      },
    },
  ]);
});

test('fetch a SCIM access list derives origin from spec.type', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({
    accessList: {
      metadata: { name: 'scim-list' },
      spec: { type: AccessListType.Scim, title: 'scim list' },
    },
  });
  const response =
    await accessManagementService.fetchAccessList('does-not-matter');
  expect(response.type).toBe(AccessListType.Scim);
  expect(response.origin).toBe(AccessListOrigin.Scim);
});

test('fetch an access list, empty response does not throw error', async () => {
  const madeResponse = {
    audit: {
      recurrence: {
        dayOfMonth: '',
        frequency: '',
      },
      nextDate: undefined,
    },
    metadata: {},
    description: '',
    grants: { roles: [], traits: {}, scopedRoles: [] },
    ownerGrants: { roles: [], traits: {}, scopedRoles: [] },
    inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
    id: '',
    type: AccessListType.Default,
    members: [],
    membersCount: undefined,
    memberListCount: undefined,
    currentUserAssignments: undefined,
    userAssignments: undefined,
    membershipRequires: { roles: [], traits: {} },
    owners: [],
    ownershipRequires: { roles: [], traits: {} },
    origin: '',
    preset: '',
    title: '',
  };

  jest.spyOn(api, 'get').mockResolvedValue({ accessList: null });
  let response =
    await accessManagementService.fetchAccessList('does-not-matter');
  expect(response).toStrictEqual(madeResponse);

  jest.spyOn(api, 'get').mockResolvedValue({
    accessList: { spec: { ownership_requires: {}, membership_requires: {} } },
  });
  response = await accessManagementService.fetchAccessList('does-not-matter');
  expect(response).toStrictEqual(madeResponse);
});

test('make an access list resolves user displays onto user rows', () => {
  const madeAccessList = makeAccessList({
    user_displays: {
      member: { primary: 'Member Name', secondary: 'Member Team' },
      adder: { primary: 'Adder Name', secondary: 'Adder Team' },
      owner: { primary: 'Owner Name', secondary: 'Owner Team' },
    },
    members: [
      {
        name: 'member',
        added_by: 'adder',
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'unknown-member',
        added_by: 'unknown-adder',
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'nested-list',
        added_by: 'adder',
        membership_kind: AccessListMemberKind.List,
      },
    ],
    spec: {
      owners: [
        {
          name: 'owner',
          membership_kind: AccessListMemberKind.User,
        },
        {
          name: 'unknown-owner',
          membership_kind: AccessListMemberKind.User,
        },
        {
          name: 'nested-owner-list',
          membership_kind: AccessListMemberKind.List,
        },
      ],
    },
  });

  expect(madeAccessList.members).toEqual([
    expect.objectContaining({
      name: 'member',
      displayPrimary: 'Member Name',
      displaySecondary: 'Member Team',
      addedByDisplayPrimary: 'Adder Name',
      addedByDisplaySecondary: 'Adder Team',
    }),
    expect.objectContaining({ name: 'unknown-member' }),
    expect.objectContaining({ name: 'nested-list' }),
  ]);
  expect(madeAccessList.members[1]).not.toHaveProperty('displayPrimary');
  expect(madeAccessList.members[1]).not.toHaveProperty('addedByDisplayPrimary');
  expect(madeAccessList.members[2]).not.toHaveProperty('displayPrimary');

  expect(madeAccessList.owners).toEqual([
    expect.objectContaining({
      name: 'owner',
      displayPrimary: 'Owner Name',
      displaySecondary: 'Owner Team',
    }),
    expect.objectContaining({ name: 'unknown-owner' }),
    expect.objectContaining({ name: 'nested-owner-list' }),
  ]);
  expect(madeAccessList.owners[1]).not.toHaveProperty('displayPrimary');
  expect(madeAccessList.owners[2]).not.toHaveProperty('displayPrimary');
});

test('make an access list without user displays leaves display fields unset', () => {
  const madeAccessList = makeAccessList({
    members: [
      {
        name: 'member',
        added_by: 'adder',
        membership_kind: AccessListMemberKind.User,
      },
    ],
    spec: {
      owners: [
        {
          name: 'owner',
          membership_kind: AccessListMemberKind.User,
        },
      ],
    },
  });

  expect(madeAccessList.members[0]).not.toHaveProperty('displayPrimary');
  expect(madeAccessList.members[0]).not.toHaveProperty('displaySecondary');
  expect(madeAccessList.members[0]).not.toHaveProperty('addedByDisplayPrimary');
  expect(madeAccessList.members[0]).not.toHaveProperty(
    'addedByDisplaySecondary'
  );
  expect(madeAccessList.owners[0]).not.toHaveProperty('displayPrimary');
  expect(madeAccessList.owners[0]).not.toHaveProperty('displaySecondary');
});

test('fetch reviews resolves reviewer info and falls back to reviewer names', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({
    reviews: [
      {
        spec: {
          notes: 'new response',
          review_date: '2026-07-17T12:00:00Z',
          reviewers: ['alice', 'unknown'],
        },
        reviewersInfo: [
          {
            username: 'alice',
            display: { primary: 'Alice Example', secondary: 'Engineering' },
          },
          { username: 'unknown' },
        ],
      },
      {
        spec: {
          notes: 'old response',
          review_date: '2026-07-16T12:00:00Z',
          reviewers: ['legacy-reviewer'],
        },
      },
    ],
    startKey: 'next-page',
  });

  const response = await accessManagementService.fetchReviews('access-list', {
    startKey: '',
    limit: 20,
  });

  expect(response).toEqual({
    reviews: [
      {
        notes: 'new response',
        reviewDate: new Date('2026-07-17T12:00:00Z'),
        reviewers: [
          {
            name: 'alice',
            displayPrimary: 'Alice Example',
            displaySecondary: 'Engineering',
          },
          { name: 'unknown' },
        ],
        raw: expect.anything(),
      },
      {
        notes: 'old response',
        reviewDate: new Date('2026-07-16T12:00:00Z'),
        reviewers: [{ name: 'legacy-reviewer' }],
        raw: expect.anything(),
      },
    ],
    startKey: 'next-page',
  });
});

test('fetch an access list', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({
    accessList: {
      metadata: {
        name: 'some-id',
        labels: {
          'okta/org': 'https://some-url',
          'teleport.internal/access-list-preset': 'short-term',
        },
        revision: '123',
      },
      membersCount: 1234,
      memberListCount: 0,
      current_user_assignments: {
        ownership_type: AccessListUserAssignmentType.EXPLICIT,
        membership_type: AccessListUserAssignmentType.UNSPECIFIED,
      },
      userAssignments: undefined,
      spec: {
        title: 'some title',
        description: 'some description',
        audit: {
          recurrence: {
            day_of_month: ReviewDayOfMonth.FifteenthDayOfMonth,
            frequency: ReviewFrequency.ThreeMonths,
          },
          next_audit_date: '2023-08-24T17:48:15.78579Z',
        },
        grants: {
          roles: ['access'],
          traits: { fruit: ['apple'] },
          scoped_roles: [
            {
              role: 'scopedaccess',
              scope: '/test',
            },
          ],
        },
        owner_grants: {
          roles: ['admin'],
          traits: { fruit: ['pro'] },
          scoped_roles: [
            {
              role: 'scopedowner',
              scope: '/test',
            },
          ],
        },
        membership_requires: {
          roles: ['intern'],
          traits: { fruit: ['banana'] },
        },
        ownership_requires: {
          roles: ['admin'],
          traits: { fruit: ['carrot'] },
        },
        owners: [
          {
            name: 'lisa',
            description: 'some description',
            ineligible_status: IneligibleStatus.UserNotExist,
            membership_kind: AccessListMemberKind.User,
          },
        ],
      },
      members: [
        {
          name: 'george',
          joined: '2023-08-24T17:48:15.78579Z',
          expires: '2023-08-24T17:48:15.78579Z',
          reason: 'some reason',
          added_by: 'llama',
          ineligible_status: IneligibleStatus.UserNotExist,
          membership_kind: AccessListMemberKind.User,
        },
      ],
    },
  });
  let response =
    await accessManagementService.fetchAccessList('does-not-matter');
  expect(response).toStrictEqual({
    id: 'some-id',
    metadata: {
      name: 'some-id',
      labels: {
        'okta/org': 'https://some-url',
        'teleport.internal/access-list-preset': 'short-term',
      },
      revision: '123',
    },
    type: AccessListType.Default,
    origin: AccessListOrigin.Okta,
    preset: 'short-term',
    title: 'some title',
    description: 'some description',
    audit: {
      recurrence: {
        dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
        frequency: ReviewFrequency.ThreeMonths,
      },
      nextDate: new Date('2023-08-24T17:48:15.78579Z'),
    },
    grants: {
      roles: ['access'],
      traits: { fruit: ['apple'] },
      scopedRoles: [
        {
          role: 'scopedaccess',
          scope: '/test',
        },
      ],
    },
    ownerGrants: {
      roles: ['admin'],
      traits: { fruit: ['pro'] },
      scopedRoles: [
        {
          role: 'scopedowner',
          scope: '/test',
        },
      ],
    },
    inheritedMemberGrants: {
      roles: [],
      traits: {},
      scopedRoles: [],
    },
    membershipRequires: {
      roles: ['intern'],
      traits: { fruit: ['banana'] },
    },
    membersCount: 1234,
    memberListCount: 0,
    currentUserAssignments: {
      ownershipType: AccessListUserAssignmentType.EXPLICIT,
      membershipType: AccessListUserAssignmentType.UNSPECIFIED,
    },
    userAssignments: undefined,
    members: [
      {
        name: 'george',
        joined: new Date('2023-08-24T17:48:15.78579Z'),
        expires: new Date('2023-08-24T17:48:15.78579Z'),
        reason: 'some reason',
        addedBy: 'llama',
        ineligibleReason: 'User does not exist',
        membershipKind: AccessListMemberKind.User,
        title: undefined,
      },
    ],
    ownershipRequires: {
      roles: ['admin'],
      traits: { fruit: ['carrot'] },
    },
    owners: [
      {
        name: 'lisa',
        description: 'some description',
        ineligibleReason: 'User does not exist',
        membershipKind: AccessListMemberKind.User,
        title: undefined,
      },
    ],
  });
});

test('fetch root scoped roles', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({
    roles: [
      {
        name: 'team-admin',
        scope: '/',
        assignableScopes: ['/dev', '/prod'],
      },
      {
        name: 'scoped-auditor',
        scope: '/',
      },
    ],
    startKey: 'next-page',
  });

  const response = await accessManagementService.fetchRootScopedRoles({
    limit: 10,
    startKey: 'page-1',
  });

  expect(api.get).toHaveBeenCalledWith(
    cfg.getRootScopedRolesUrl({ limit: 10, startKey: 'page-1' }),
    undefined
  );
  expect(response).toStrictEqual({
    roles: [
      {
        name: 'team-admin',
        scope: '/',
        assignableScopes: ['/dev', '/prod'],
      },
      {
        name: 'scoped-auditor',
        scope: '/',
        assignableScopes: [],
      },
    ],
    startKey: 'next-page',
  });
});

describe('update an access list', () => {
  const originalAccessList: AccessList = {
    id: 'some-id',
    type: AccessListType.Default,
    title: 'some title',
    description: 'some description',
    metadata: {
      labels: {},
      name: '',
      revision: '',
    },
    audit: {
      recurrence: {
        dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
        frequency: ReviewFrequency.ThreeMonths,
      },
      nextDate: new Date('2023-08-24T17:48:15.78579Z'),
    },
    grants: {
      roles: ['access'],
      traits: { fruit: ['apple'] },
      scopedRoles: [
        {
          role: 'scopedaccess',
          scope: '/test',
        },
      ],
    },
    ownerGrants: {
      roles: ['admin'],
      traits: { status: ['pro'] },
      scopedRoles: [
        {
          role: 'scopedowner',
          scope: '/test',
        },
      ],
    },
    membershipRequires: {
      roles: ['intern'],
      traits: { fruit: ['banana'] },
    },
    members: [
      {
        name: 'george',
        joined: new Date('2023-08-24T17:48:15.78579Z'),
        expires: new Date('2023-08-24T17:48:15.78579Z'),
        reason: 'some reason',
        addedBy: 'llama',
        ineligibleReason: 'some member ineligible reason',
        membershipKind: AccessListMemberKind.User,
      },
    ],
    ownershipRequires: {
      roles: ['admin'],
      traits: { fruit: ['carrot'] },
    },
    owners: [
      {
        name: 'lisa',
        description: 'some description',
        ineligibleReason: 'some owner ineligible reason',
        membershipKind: AccessListMemberKind.User,
        title: '',
      },
    ],
    inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
  };

  const madeForAccessListUpdate: UpsertAccessListRequest = {
    type: AccessListType.Default,
    title: 'some title',
    description: 'some description',
    audit: {
      recurrence: {
        day_of_month: ReviewDayOfMonth.FifteenthDayOfMonth,
        frequency: convertReviewFrequencyIntoBackendParsableValue(
          ReviewFrequency.ThreeMonths
        ),
      },
      next_audit_date: new Date('2023-08-24T17:48:15.78579Z'),
    },
    grants: {
      roles: ['access'],
      traits: { fruit: ['apple'] },
      scoped_roles: [
        {
          role: 'scopedaccess',
          scope: '/test',
        },
      ],
    },
    owner_grants: {
      roles: ['admin'],
      traits: { status: ['pro'] },
      scoped_roles: [
        {
          role: 'scopedowner',
          scope: '/test',
        },
      ],
    },
    membership_requires: {
      roles: ['intern'],
      traits: { fruit: ['banana'] },
    },
    members: [
      {
        name: 'george',
        joined: new Date('2023-08-24T17:48:15.78579Z'),
        expires: new Date('2023-08-24T17:48:15.78579Z'),
        reason: 'some reason',
        added_by: 'llama',
        membership_kind: AccessListMemberKind.User,
      },
    ],
    ownership_requires: {
      roles: ['admin'],
      traits: { fruit: ['carrot'] },
    },
    owners: [
      {
        name: 'lisa',
        description: 'some description',
        membership_kind: AccessListMemberKind.User,
      },
    ],
  };

  [
    {
      case: 'empty request should use original',
      reqToUpdate: {},
      constructed: madeForAccessListUpdate,
    },
    {
      case: 'modify title and description',
      reqToUpdate: {
        title: 'some other title',
        description: 'some other description',
      },
      constructed: {
        ...madeForAccessListUpdate,
        title: 'some other title',
        description: 'some other description',
      },
    },
    {
      case: 'modify description into empty',
      reqToUpdate: {
        description: '',
      },
      constructed: {
        ...madeForAccessListUpdate,
        description: '',
      },
    },
    {
      case: 'modify audit',
      reqToUpdate: {
        audit: {
          nextDate: new Date('2024-08-24T17:48:15.78579Z'),
          recurrence: {
            dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
            frequency: ReviewFrequency.OneYear,
          },
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        audit: {
          next_audit_date: new Date('2024-08-24T17:48:15.78579Z'),
          recurrence: {
            day_of_month: ReviewDayOfMonth.LastDayOfMonth,
            frequency: '12m',
          },
        },
      },
    },
    {
      case: 'modify grants',
      reqToUpdate: {
        grants: {
          roles: ['different-role1', 'different-role2'],
          traits: { different1: ['different'], different2: ['different2'] },
          scopedRoles: [],
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        grants: {
          roles: ['different-role1', 'different-role2'],
          traits: { different1: ['different'], different2: ['different2'] },
          scoped_roles: [],
        },
      },
    },
    {
      case: 'modify owner_grants',
      reqToUpdate: {
        ownerGrants: {
          roles: ['different-role5', 'different-role6'],
          traits: { different1: ['different1'], different2: ['different3'] },
          scopedRoles: [],
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        owner_grants: {
          roles: ['different-role5', 'different-role6'],
          traits: { different1: ['different1'], different2: ['different3'] },
          scoped_roles: [],
        },
      },
    },
    {
      case: 'modify members',
      reqToUpdate: {
        members: [
          {
            name: 'diff1',
            joined: new Date('2024-08-24T17:48:15.78579Z'),
            expires: new Date('2023-08-24T17:48:15.78579Z'),
            reason: '',
            addedBy: 'diff',
            ineligibleReason: 'some reason',
            membershipKind: AccessListMemberKind.User,
          },
        ],
      },
      constructed: {
        ...madeForAccessListUpdate,
        members: [
          {
            name: 'diff1',
            joined: new Date('2024-08-24T17:48:15.78579Z'),
            expires: new Date('2023-08-24T17:48:15.78579Z'),
            reason: '',
            added_by: 'diff',
            membership_kind: AccessListMemberKind.User,
          },
        ],
      },
    },
    {
      case: 'modify owners',
      reqToUpdate: {
        owners: [
          {
            name: 'diff1',
            description: 'diff description',
            membershipKind: AccessListMemberKind.User,
          },
        ],
      },
      constructed: {
        ...madeForAccessListUpdate,
        owners: [
          {
            name: 'diff1',
            description: 'diff description',
            membership_kind: AccessListMemberKind.User,
          },
        ],
      },
    },
    {
      case: 'modify membership requires',
      reqToUpdate: {
        membershipRequires: {
          roles: ['diff-role'],
          traits: { diff: ['diff-trait'] },
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        membership_requires: {
          roles: ['diff-role'],
          traits: { diff: ['diff-trait'] },
        },
      },
    },
    {
      case: 'modify ownership requires',
      reqToUpdate: {
        ownershipRequires: {
          roles: ['diff-role'],
          traits: { diff: ['diff-trait'] },
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        ownership_requires: {
          roles: ['diff-role'],
          traits: { diff: ['diff-trait'] },
        },
      },
    },
    {
      case: 'modify multi fields',
      reqToUpdate: {
        ownershipRequires: {
          roles: ['diff-role'],
          traits: { diff: ['diff-trait'] },
        },
        audit: {
          nextDate: new Date('2024-08-24T17:48:15.78579Z'),
          recurrence: {
            dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
            frequency: ReviewFrequency.OneMonth,
          },
        },
        owners: [
          {
            name: 'diff1',
            description: 'diff description',
            membershipKind: AccessListMemberKind.User,
          },
        ],
      },
      constructed: {
        ...madeForAccessListUpdate,
        ownership_requires: {
          roles: ['diff-role'],
          traits: { diff: ['diff-trait'] },
        },
        audit: {
          next_audit_date: new Date('2024-08-24T17:48:15.78579Z'),
          recurrence: {
            day_of_month: ReviewDayOfMonth.FirstDayOfMonth,
            frequency: '1m',
          },
        },
        owners: [
          {
            name: 'diff1',
            description: 'diff description',
            membership_kind: AccessListMemberKind.User,
          },
        ],
      },
    },
  ].forEach(tc => {
    jest.spyOn(api, 'put').mockResolvedValue({}); // response doesn't matter
    test(`case: ${tc.case}`, async () => {
      await accessManagementService.updateAccessList({
        req: tc.reqToUpdate,
        original: originalAccessList,
      });
      expect(api.put).toHaveBeenCalledWith(
        cfg.getAccessManagementListUrl(originalAccessList.id),
        tc.constructed
      );
      jest.clearAllMocks();
    });
  });
});
