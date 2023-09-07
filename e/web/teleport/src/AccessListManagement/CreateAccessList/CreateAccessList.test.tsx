import {
  getEligibleUsers,
  getEligibleUsersAmongSelectedUsers,
} from './CreateAccessList';

test('getEligibleUsers: empty', async () => {
  expect(getEligibleUsers([], {}, [])).toStrictEqual([]);

  expect(
    getEligibleUsers([], {}, [
      { label: 'foo', value: { roles: ['access'] } as any },
      { label: 'bar', value: { roles: ['editor'] } },
      { label: 'baz', value: { roles: ['access'] } },
    ])
  ).toStrictEqual([]);
});

test('getEligibleUsers: match by roles only (empty traits)', async () => {
  expect(
    getEligibleUsers([{ label: 'access', value: 'access' }], {}, [
      { label: 'foo', value: { roles: ['access'] } as any },
      { label: 'bar', value: { roles: ['editor'] } },
      { label: 'baz', value: { roles: ['access'] } },
    ])
  ).toStrictEqual([
    { label: 'foo', value: { roles: ['access'] } },
    { label: 'baz', value: { roles: ['access'] } },
  ]);
});

test('getEligibleUsers: match by traits only (empty roles)', async () => {
  expect(
    getEligibleUsers([], { fruit: { apple: true } }, [
      {
        label: 'foo',
        value: { allTraits: { fruit: ['apple'] } } as any,
      },
      { label: 'bar', value: { allTraits: { fruit: ['banana'] } } },
      { label: 'baz', value: { allTraits: { fruit: ['apple'] } } },
    ])
  ).toStrictEqual([
    { label: 'foo', value: { allTraits: { fruit: ['apple'] } } },
    { label: 'baz', value: { allTraits: { fruit: ['apple'] } } },
  ]);
});

test('getEligibleUsers: match by both roles and traits', async () => {
  expect(
    getEligibleUsers(
      [{ label: 'access', value: 'access' }],
      { fruit: { apple: true } },
      [
        {
          label: 'foo',
          value: { roles: ['access'], allTraits: {} } as any,
        },
        {
          label: 'bar',
          value: {
            roles: ['access'],
            allTraits: { fruit: ['apple'] },
          },
        },
        {
          label: 'baz',
          value: { roles: [], allTraits: { fruit: ['apple'] } },
        },
        {
          label: 'lux',
          value: {
            roles: ['access'],
            allTraits: {},
          },
        },
        {
          label: 'qux',
          value: {
            roles: ['editor', 'access'],
            allTraits: { fruit: ['apple', 'banana'] },
          },
        },
      ]
    )
  ).toStrictEqual([
    {
      label: 'bar',
      value: { roles: ['access'], allTraits: { fruit: ['apple'] } },
    },
    {
      label: 'qux',
      value: {
        roles: ['editor', 'access'],
        allTraits: { fruit: ['apple', 'banana'] },
      },
    },
  ]);
});

test('getEligibleUsersAmongSelectedUsers: empty', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({ eligibleUsers: [], selectedUsers: [] })
  ).toStrictEqual([]);
});

test('getEligibleUsersAmongSelectedUsers: no eligible users', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [],
      selectedUsers: [{ value: { name: 'foo' } } as any],
    })
  ).toStrictEqual([]);
});

test('getEligibleUsersAmongSelectedUsers: no selected users are eligible', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [{ value: { name: 'bar' } } as any],
      selectedUsers: [{ value: { name: 'foo' } } as any],
    })
  ).toStrictEqual([]);
});

test('getEligibleUsersAmongSelectedUsers', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [
        { value: { name: 'foo' } } as any,
        { value: { name: 'baz' } },
      ],
      selectedUsers: [
        { value: { name: 'foo' } } as any,
        { value: { name: 'bar' } },
        { value: { name: 'baz' } },
      ],
    })
  ).toStrictEqual([
    { value: { name: 'foo' } } as any,
    { value: { name: 'baz' } },
  ]);
});

test('getEligibleUsersAmongSelectedUsers ignore options that contain string as a value', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [
        { value: { name: 'foo' } } as any,
        { value: { name: 'baz' } },
      ],
      selectedUsers: [
        { value: { name: 'foo' } } as any,
        { value: 'manual-user-1' },
        { value: { name: 'bar' } },
        { value: { name: 'baz' } },
        { value: 'manual-user-2' },
      ],
    })
  ).toStrictEqual([
    { value: { name: 'foo' } } as any,
    { value: { name: 'baz' } },
  ]);
});
