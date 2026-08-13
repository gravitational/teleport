import { renderHook } from '@testing-library/react';
import { PropsWithChildren } from 'react';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import type { User } from 'teleport/services/user';

import { useUserOptions, useUsersNoOptionsMessage } from './hooks';

afterEach(() => {
  jest.restoreAllMocks();
});

const collector = (users: User[]) =>
  users.map(u => ({ value: u, label: u.name }));

function renderUserOptions(ctx: ReturnType<typeof createTeleportContextE>) {
  const wrapper = ({ children }: PropsWithChildren) => (
    <ContextProvider ctx={ctx}>{children}</ContextProvider>
  );

  return renderHook(() => useUserOptions(collector), { wrapper });
}

test('loadOptions returns users when allowed to list them', async () => {
  const ctx = createTeleportContextE();
  const fetchUsersV2 = jest
    .spyOn(ctx.userService, 'fetchUsersV2')
    .mockResolvedValue({ items: [{ name: 'alice', roles: [] }], startKey: '' });

  const { result } = renderUserOptions(ctx);

  await expect(result.current.loadOptions('')).resolves.toEqual([
    { value: { name: 'alice', roles: [] }, label: 'alice' },
  ]);
  expect(fetchUsersV2).toHaveBeenCalledWith({ search: '', limit: 50 });
  expect(result.current.canListUsers).toBe(true);
});

test.each([
  ['read', { list: true, read: false }],
  ['list', { list: false, read: true }],
])(
  'loadOptions does not request users without %s access',
  async (_, access) => {
    const ctx = createTeleportContextE();
    const userAccess = ctx.storeUser.getUserAccess();
    jest
      .spyOn(ctx.storeUser, 'getUserAccess')
      .mockReturnValue({ ...userAccess, ...access });
    const fetchUsersV2 = jest.spyOn(ctx.userService, 'fetchUsersV2');

    const { result } = renderUserOptions(ctx);

    await expect(result.current.loadOptions('')).resolves.toEqual([]);
    expect(fetchUsersV2).not.toHaveBeenCalled();
    expect(result.current.canListUsers).toBe(false);
  }
);

// The empty list has to explain itself, otherwise it reads as "this cluster
// has no users".
test.each([
  [true, 'Type a username and press enter'],
  [false, 'You do not have permission to list users'],
])('noOptionsMessage with canListUsers=%s', (canListUsers, expected) => {
  const { result } = renderHook(() => useUsersNoOptionsMessage(canListUsers));

  expect(result.current()).toEqual(expected);
});

test('noOptionsMessage defaults to the generic message', () => {
  const { result } = renderHook(() => useUsersNoOptionsMessage());

  expect(result.current()).toEqual('Type a username and press enter');
});
