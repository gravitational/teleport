import { renderHook } from '@testing-library/react';
import { PropsWithChildren } from 'react';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessListMemberKind,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';

import { useFetch } from './useFetch';

afterEach(() => {
  jest.restoreAllMocks();
});

test('fetchUsersOptions preserves user display fields', async () => {
  const ctx = createTeleportContextE();
  const fetchUsersV2 = jest
    .spyOn(ctx.userService, 'fetchUsersV2')
    .mockResolvedValue({
      items: [
        {
          name: 'alice',
          displayPrimary: 'Alice Liddell',
          displaySecondary: 'alice@example.com',
          roles: [],
        },
      ],
      startKey: '',
    });
  jest
    .spyOn(accessManagementService, 'fetchAccessListsV2')
    .mockResolvedValue({ agents: [] });

  const wrapper = ({ children }: PropsWithChildren) => (
    <ContextProvider ctx={ctx}>{children}</ContextProvider>
  );

  const { result } = renderHook(() => useFetch(), { wrapper });

  await expect(result.current.fetchUsersOptions('alice')).resolves.toEqual([
    {
      label: 'alice',
      value: {
        membershipKind: AccessListMemberKind.User,
        name: 'alice',
        displayPrimary: 'Alice Liddell',
        displaySecondary: 'alice@example.com',
      },
    },
  ]);
  expect(fetchUsersV2).toHaveBeenCalledWith({
    search: 'alice',
    limit: 50,
    searchMode: 'identity',
  });
});
