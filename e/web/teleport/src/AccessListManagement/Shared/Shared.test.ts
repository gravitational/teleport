import { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import {
  AccessListMemberKind,
  AccessListOrigin,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';

import {
  filterAccessLists,
  matchRoles,
  matchTraits,
  sortAccessLists,
} from './Shared';

describe('matchRoles', () => {
  // eslint-disable-next-line jest/require-hook
  [
    {
      case: 'empty',
      rolesRequired: [],
      userOptions: [],
      output: [],
    },
    {
      case: 'no user options should return empty array',
      rolesRequired: ['access', 'editor'],
      userOptions: [],
      output: [],
    },
    {
      case: 'no rolesRequired should return all users',
      rolesRequired: [],
      userOptions: [
        { label: 'foo', value: { roles: ['access'] } },
        { label: 'bar', value: { roles: ['editor'] } },
      ],
      output: [
        { label: 'foo', value: { roles: ['access'] } },
        { label: 'bar', value: { roles: ['editor'] } },
      ],
    },
    {
      case: 'no users match required roles',
      rolesRequired: ['admin'],
      userOptions: [
        { label: 'foo', value: { roles: ['access'] } },
        { label: 'bar', value: { roles: ['editor'] } },
      ],
      output: [],
    },
    {
      case: 'all users match required roles',
      rolesRequired: ['access'],
      userOptions: [
        { label: 'foo', value: { roles: ['editor', 'access'] } },
        { label: 'bar', value: { roles: ['access'] } },
        { label: 'baz', value: { roles: ['editor', 'admin', 'access'] } },
        { label: 'qux', value: { roles: ['access', 'admin'] } },
      ],
      output: [
        { label: 'foo', value: { roles: ['editor', 'access'] } },
        { label: 'bar', value: { roles: ['access'] } },
        { label: 'baz', value: { roles: ['editor', 'admin', 'access'] } },
        { label: 'qux', value: { roles: ['access', 'admin'] } },
      ],
    },
    {
      case: 'require 1 role',
      rolesRequired: ['access'],
      userOptions: [
        { label: 'foo', value: { roles: ['access'] } },
        { label: 'bar', value: { roles: ['editor'] } },
        { label: 'baz', value: { roles: ['editor'] } },
        { label: 'qux', value: { roles: ['access'] } },
      ],
      output: [
        { label: 'foo', value: { roles: ['access'] } },
        { label: 'qux', value: { roles: ['access'] } },
      ],
    },
    {
      case: 'require 2 roles',
      rolesRequired: ['access', 'editor'],
      userOptions: [
        { label: 'foo', value: { roles: ['admin', 'access', 'editor'] } },
        { label: 'bar', value: { roles: ['editor'] } },
        {
          label: 'baz',
          value: { roles: ['auditor', 'editor', 'admin', 'access'] },
        },
        { label: 'qux', value: { roles: ['access'] } },
      ],
      output: [
        { label: 'foo', value: { roles: ['admin', 'access', 'editor'] } },
        {
          label: 'baz',
          value: { roles: ['auditor', 'editor', 'admin', 'access'] },
        },
      ],
    },
  ].forEach(tc => {
    test(`case: ${tc.case}`, () => {
      const obj = matchRoles(tc.rolesRequired, tc.userOptions as any);
      expect(obj).toStrictEqual(tc.output);
    });
  });
});

describe('matchTraits', () => {
  // eslint-disable-next-line jest/require-hook
  [
    {
      case: 'empty',
      traitsRequired: {},
      userOptions: [],
      output: [],
    },
    {
      case: 'no user options should return empty array',
      traitsRequired: { fruit: ['apple'] },
      userOptions: [],
      output: [],
    },
    {
      case: 'no traitsRequired should return all users',
      traitsRequired: {},
      userOptions: [
        { label: 'foo', value: { allTraits: { fruit: ['banana'] } } },
        { label: 'bar', value: { allTraits: { drink: ['water'] } } },
      ],
      output: [
        { label: 'foo', value: { allTraits: { fruit: ['banana'] } } },
        { label: 'bar', value: { allTraits: { drink: ['water'] } } },
      ],
    },
    {
      case: 'no users match required traits',
      traitsRequired: { fruit: ['apple'] },
      userOptions: [
        { label: 'foo', value: { allTraits: { fruit: ['banana'] } } },
        { label: 'bar', value: { allTraits: { drink: ['water'] } } },
      ],
      output: [],
    },
    {
      case: 'all users match required allTraits',
      traitsRequired: { fruit: ['apple'] },
      userOptions: [
        { label: 'foo', value: { allTraits: { fruit: ['apple'] } } },
        {
          label: 'bar',
          value: {
            allTraits: { drink: ['water', 'apple'], fruit: ['apple'] },
          },
        },
        {
          label: 'baz',
          value: {
            allTraits: {
              drink: ['apple'],
              fruit: ['banana', 'carrot', 'apple'],
              month: ['apple'],
            },
          },
        },
      ],
      output: [
        { label: 'foo', value: { allTraits: { fruit: ['apple'] } } },
        {
          label: 'bar',
          value: {
            allTraits: { drink: ['water', 'apple'], fruit: ['apple'] },
          },
        },
        {
          label: 'baz',
          value: {
            allTraits: {
              drink: ['apple'],
              fruit: ['banana', 'carrot', 'apple'],
              month: ['apple'],
            },
          },
        },
      ],
    },
    {
      case: 'require 1 trait (require all values)',
      traitsRequired: { fruit: ['apple', 'banana'] },
      userOptions: [
        { label: 'foo', value: { allTraits: { fruit: ['banana'] } } },
        {
          label: 'bar',
          value: {
            allTraits: {
              drink: ['water', 'latte'],
              fruit: ['apple', 'banana'],
            },
          },
        },
        {
          label: 'baz',
          value: { allTraits: { drink: ['apple'], fruit: ['apple'] } },
        },
        {
          label: 'qux',
          value: {
            allTraits: { fruit: ['banana', 'carrot', 'apple'] },
          },
        },
      ],
      output: [
        {
          label: 'bar',
          value: {
            allTraits: {
              drink: ['water', 'latte'],
              fruit: ['apple', 'banana'],
            },
          },
        },
        {
          label: 'qux',
          value: {
            allTraits: { fruit: ['banana', 'carrot', 'apple'] },
          },
        },
      ],
    },
    {
      case: 'require 2 traits (require all values)',
      traitsRequired: {
        fruit: ['apple', 'banana'],
        drink: ['coffee', 'water', 'latte'],
      },
      userOptions: [
        {
          label: 'foo',
          value: {
            allTraits: {
              drink: ['water', 'latte', 'coffee', 'coffee2'],
              fruit: ['banana', 'apple'],
            },
          },
        },
        {
          label: 'bar',
          value: {
            allTraits: {
              drink: ['coffee', 'latte'],
              fruit: ['banana'],
            },
          },
        },
        {
          label: 'baz',
          value: { allTraits: { fruit: ['apple'], month: ['apple'] } },
        },
        {
          label: 'qux',
          value: {
            allTraits: {
              fruit: ['banana', 'carrot', 'apple'],
              drink: ['coffee', 'water'],
            },
          },
        },
      ],
      output: [
        {
          label: 'foo',
          value: {
            allTraits: {
              drink: ['water', 'latte', 'coffee', 'coffee2'],
              fruit: ['banana', 'apple'],
            },
          },
        },
      ],
    },
  ].forEach(tc => {
    test(`case: ${tc.case}`, () => {
      const obj = matchTraits(tc.traitsRequired, tc.userOptions as any);
      expect(obj).toStrictEqual(tc.output);
    });
  });
});

describe('Access List Management Shared', () => {
  const mockAccessLists = [
    {
      id: '1',
      title: 'Admin List',
      description: 'Admins and superusers',
      owners: [
        { name: 'Alice', membershipKind: AccessListMemberKind.User },
        { name: 'Bob', membershipKind: AccessListMemberKind.User },
      ],
      grants: { roles: ['admin', 'superuser'], traits: {}, traitList: [] },
      inheritedMemberGrants: { roles: [], traits: {} },
      ownerGrants: { roles: [], traits: {}, traitList: [] },
      ownershipRequires: { roles: [], traits: {} },
      audit: {
        recurrence: {
          dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
          frequency: ReviewFrequency.OneYear,
        },
        nextDate: new Date(Date.now() + 1000 * 60 * 60 * 24 * 365),
      },
      origin: AccessListOrigin.Unspecified,
      members: Array(15).fill({}),
    },
    {
      id: '2',
      title: 'User List',
      description: 'Regular users',
      owners: [{ name: 'Charlie', membershipKind: AccessListMemberKind.User }],
      grants: { roles: ['user'], traits: {}, traitList: [] },
      inheritedMemberGrants: { roles: [], traits: {} },
      ownerGrants: { roles: [], traits: {}, traitList: [] },
      ownershipRequires: { roles: [], traits: {} },
      audit: {
        recurrence: {
          dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
          frequency: ReviewFrequency.OneYear,
        },
        nextDate: new Date(Date.now() + 1000 * 60 * 60 * 24 * 180),
      },
      origin: AccessListOrigin.Okta,
      members: Array(65).fill({}),
    },
    {
      id: '3',
      title: 'Support List',
      description: 'Support staff and users',
      owners: [{ name: 'Bob', membershipKind: AccessListMemberKind.User }],
      grants: { roles: ['support', 'user'], traits: {}, traitList: [] },
      inheritedMemberGrants: { roles: [], traits: {} },
      ownerGrants: { roles: [], traits: {}, traitList: [] },
      ownershipRequires: { roles: [], traits: {} },
      audit: {
        recurrence: {
          dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
          frequency: ReviewFrequency.OneYear,
        },
        nextDate: new Date(Date.now() + 1000 * 60 * 60 * 24 * 90),
      },
      origin: AccessListOrigin.Okta,
      members: Array(25).fill({}),
    },
    {
      id: '4',
      title: 'AWS IAM Identity Center List',
      description: 'AWS groups',
      owners: [{ name: 'AWSDev', membershipKind: AccessListMemberKind.User }],
      grants: { roles: ['devops'], traits: {}, traitList: [] },
      inheritedMemberGrants: { roles: [], traits: {} },
      ownerGrants: { roles: [], traits: {}, traitList: [] },
      ownershipRequires: { roles: [], traits: {} },
      audit: {
        recurrence: {
          dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
          frequency: ReviewFrequency.OneYear,
        },
        nextDate: new Date(Date.now() + 1000 * 60 * 60 * 24 * 90),
      },
      origin: AccessListOrigin.AwsIdentityCenter,
      members: Array(1).fill({}),
    },
  ].map(list => {
    return {
      ...list,
      membersCount: list.members.length - 1,
      memberListCount: 1,
      auditNextDate: list.audit.nextDate,
      needsReviewBy: list.audit.nextDate,
    };
  }) satisfies AccessListWithModifiedGrants[];

  describe('sortAccessLists', () => {
    it('sorts by title ascending', () => {
      const result = sortAccessLists(mockAccessLists, {
        fieldName: 'title',
        dir: 'ASC',
      });
      expect(result.map(r => r.title)).toEqual([
        'Admin List',
        'AWS IAM Identity Center List',
        'Support List',
        'User List',
      ]);
    });

    it('sorts by title descending', () => {
      const result = sortAccessLists(mockAccessLists, {
        fieldName: 'title',
        dir: 'DESC',
      });
      expect(result.map(r => r.title)).toEqual([
        'User List',
        'Support List',
        'AWS IAM Identity Center List',
        'Admin List',
      ]);
    });

    it('sorts by a numeric field with nullish handling', () => {
      const lists = [
        { ...mockAccessLists[0], auditNextDate: null },
        { ...mockAccessLists[1] },
        { ...mockAccessLists[2] },
      ];
      const result = sortAccessLists(lists, {
        fieldName: 'auditNextDate',
        dir: 'ASC',
      });
      expect(result.map(r => r.auditNextDate)).toEqual([
        lists[2].auditNextDate,
        lists[1].auditNextDate,
        null,
      ]);
    });

    it('uses title as tiebreaker when fields match', () => {
      const lists = [
        { ...mockAccessLists[0], id: '1', title: 'Foo title' },
        { ...mockAccessLists[1], id: '1', title: 'Bar list' },
      ];
      const result = sortAccessLists(lists, { fieldName: 'id', dir: 'ASC' });
      expect(result.map(r => r.title)).toEqual(['Bar list', 'Foo title']);
    });

    it('does not mutate the original array', () => {
      const original = [...mockAccessLists];
      sortAccessLists(mockAccessLists, { fieldName: 'title', dir: 'ASC' });
      expect(mockAccessLists).toEqual(original);
    });
  });

  describe('filterAccessLists', () => {
    it('filters by searchValue matching title', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: 'Admin',
        filterValue: {},
      });
      expect(result).toEqual([mockAccessLists[0]]);
    });

    it('filters by searchValue matching owners', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: 'Bob',
        filterValue: {},
      });
      expect(result).toEqual([mockAccessLists[0], mockAccessLists[2]]);
    });

    it('filters by searchValue matching roles', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: 'admin',
        filterValue: {},
      });
      expect(result).toEqual([mockAccessLists[0]]);
    });

    it('filters by source (Okta)', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: '',
        filterValue: { source: ['okta'] },
      });
      expect(result).toEqual([mockAccessLists[1], mockAccessLists[2]]);
    });

    it('filters by owners', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: '',
        filterValue: { owners: ['Alice'] },
      });
      expect(result).toEqual([mockAccessLists[0]]);
    });

    it('filters by roles', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: '',
        filterValue: { roles: ['superuser'] },
      });
      expect(result).toEqual([mockAccessLists[0]]);
    });

    it('applies multiple filters together', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: 'support',
        filterValue: { source: ['okta'], roles: ['user'] },
      });
      expect(result).toEqual([mockAccessLists[2]]);
    });

    it('returns all access lists when no filters are applied', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: '',
        filterValue: {},
      });
      expect(result).toEqual(mockAccessLists);
    });

    it('filters by source (AWS IAM Identity Center)', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: '',
        filterValue: { source: ['aws-identity-center'] },
      });
      expect(result).toEqual([mockAccessLists[3]]);
    });

    it('search by source (AWS IAM Identity Center)', () => {
      const result = filterAccessLists({
        accessLists: mockAccessLists,
        searchValue: 'aws',
        filterValue: {},
      });
      expect(result).toEqual([mockAccessLists[3]]);
    });
  });
});
