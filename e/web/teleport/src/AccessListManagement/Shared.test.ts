import {
  calculateMonthsDaysFromDuration,
  getFormattedDate,
  matchRoles,
  matchTraits,
} from './Shared';

describe('calculateMonthsDaysFromDuration', () => {
  // eslint-disable-next-line jest/require-hook
  [
    { duration: '0h0m0s', output: { months: 0, days: 0 } },
    { duration: '730h0m0s', output: { months: 1, days: 0 } },
    { duration: '1460h', output: { months: 2, days: 0 } },
    { duration: '2190h', output: { months: 3, days: 0 } },
    { duration: '2920h', output: { months: 4, days: 0 } },
    { duration: '3650h', output: { months: 5, days: 0 } },
    { duration: '4380h', output: { months: 6, days: 0 } },
    { duration: '', output: { months: 0, days: 0 } },
    { duration: '0h4320m', output: { months: 0, days: 3 } },
    { duration: '0h0m259200s', output: { months: 0, days: 3 } },
    { duration: '4380h4320m259200s', output: { months: 6, days: 6 } },
  ].forEach(tc => {
    test(`duration: ${tc.duration}`, () => {
      const obj = calculateMonthsDaysFromDuration(tc.duration);
      expect(obj).toStrictEqual(tc.output);
    });
  });
});

test('getFormattedDate', async () => {
  expect(getFormattedDate(null)).toBe('');
  expect(getFormattedDate(new Date('0001-01-01T00:00:00Z'))).toBe('');
  expect(getFormattedDate(undefined)).toBe('');
  expect(getFormattedDate(new Date(null))).toBe('');
  expect(getFormattedDate(new Date('2023-08-24T17:48:15.78579Z'))).toBe(
    '08/24/2023'
  );
});

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
      traitsRequired: { fruit: { apple: true } },
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
      traitsRequired: { fruit: { apple: true } },
      userOptions: [
        { label: 'foo', value: { allTraits: { fruit: ['banana'] } } },
        { label: 'bar', value: { allTraits: { drink: ['water'] } } },
      ],
      output: [],
    },
    {
      case: 'all users match required allTraits',
      traitsRequired: { fruit: { apple: true } },
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
      case: 'require 1 trait',
      traitsRequired: { fruit: { apple: true } },
      userOptions: [
        { label: 'foo', value: { allTraits: { fruit: ['banana'] } } },
        {
          label: 'bar',
          value: {
            allTraits: { drink: ['water', 'latte'], fruit: ['apple'] },
          },
        },
        {
          label: 'baz',
          value: { allTraits: { drink: ['apple'], month: ['apple'] } },
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
            allTraits: { drink: ['water', 'latte'], fruit: ['apple'] },
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
      case: 'require 2 traits',
      traitsRequired: { fruit: { apple: true }, drink: { coffee: true } },
      userOptions: [
        {
          label: 'foo',
          value: {
            allTraits: {
              drink: ['water', 'latte', 'coffee'],
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
              drink: ['coffee'],
            },
          },
        },
      ],
      output: [
        {
          label: 'foo',
          value: {
            allTraits: {
              drink: ['water', 'latte', 'coffee'],
              fruit: ['banana', 'apple'],
            },
          },
        },
        {
          label: 'qux',
          value: {
            allTraits: {
              fruit: ['banana', 'carrot', 'apple'],
              drink: ['coffee'],
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
