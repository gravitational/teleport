import { matchRoles, matchTraits } from './Shared';

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
