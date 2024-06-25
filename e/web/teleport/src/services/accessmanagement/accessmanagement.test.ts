import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import {
  accessManagementService,
  convertReviewFrequencyIntoBackendParsableValue,
} from './accessmanagement';
import {
  AccessList,
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
      isOkta: false,
      title: '',
      description: '',
      owners: [],
      members: [],
      membersCount: undefined,
      grants: {
        roles: [],
        traits: {},
      },
      ownerGrants: {
        roles: [],
        traits: {},
      },
      audit: {
        recurrence: {
          dayOfMonth: 0,
          frequency: 0,
        },
        nextDate: undefined,
      },
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

test('fetch an access list, empty response does not throw error', async () => {
  const madeResponse = {
    audit: {
      recurrence: {
        dayOfMonth: 0,
        frequency: 0,
      },
      nextDate: undefined,
    },
    description: '',
    grants: { roles: [], traits: {} },
    ownerGrants: { roles: [], traits: {} },
    id: '',
    members: [],
    membersCount: undefined,
    membershipRequires: { roles: [], traits: {} },
    owners: [],
    ownershipRequires: { roles: [], traits: {} },
    isOkta: false,
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

test('fetch an access list', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({
    accessList: {
      metadata: {
        name: 'some-id',
        labels: {
          'okta/org': 'https://some-url',
        },
      },
      membersCount: 1234,
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
        },
        owner_grants: {
          roles: ['admin'],
          traits: { fruit: ['pro'] },
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
        },
      ],
    },
  });
  let response =
    await accessManagementService.fetchAccessList('does-not-matter');
  expect(response).toStrictEqual({
    id: 'some-id',
    isOkta: true,
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
    },
    ownerGrants: {
      roles: ['admin'],
      traits: { fruit: ['pro'] },
    },
    membershipRequires: {
      roles: ['intern'],
      traits: { fruit: ['banana'] },
    },
    membersCount: 1234,
    members: [
      {
        name: 'george',
        joined: new Date('2023-08-24T17:48:15.78579Z'),
        expires: new Date('2023-08-24T17:48:15.78579Z'),
        reason: 'some reason',
        addedBy: 'llama',
        ineligibleReason: 'User does not exist',
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
      },
    ],
  });
});

describe('update an access list', () => {
  const originalAccessList: AccessList = {
    id: 'some-id',
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
    },
    ownerGrants: {
      roles: ['admin'],
      traits: { status: ['pro'] },
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
      },
    ],
  };

  const madeForAccessListUpdate: UpsertAccessListRequest = {
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
    },
    owner_grants: {
      roles: ['admin'],
      traits: { status: ['pro'] },
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
      },
    ],
  };

  // eslint-disable-next-line jest/require-hook
  [
    {
      case: 'empty request should use original',
      reqToUpdate: {},
      constructed: madeForAccessListUpdate,
    },
    {
      case: 'modify title but cannot change description',
      reqToUpdate: {
        title: 'some other title',
        description: 'cannot change description',
      },
      constructed: {
        ...madeForAccessListUpdate,
        title: 'some other title',
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
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        grants: {
          roles: ['different-role1', 'different-role2'],
          traits: { different1: ['different'], different2: ['different2'] },
        },
      },
    },
    {
      case: 'modify owner_grants',
      reqToUpdate: {
        ownerGrants: {
          roles: ['different-role5', 'different-role6'],
          traits: { different1: ['different1'], different2: ['different3'] },
        },
      },
      constructed: {
        ...madeForAccessListUpdate,
        owner_grants: {
          roles: ['different-role5', 'different-role6'],
          traits: { different1: ['different1'], different2: ['different3'] },
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
          },
        ],
      },
      constructed: {
        ...madeForAccessListUpdate,
        owners: [
          {
            name: 'diff1',
            description: 'diff description',
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
