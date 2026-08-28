import { useCallback } from 'react';

import {
  AccessListMemberKind,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'teleport/useTeleport';

import { HybridUserOption } from './Shared/Shared';

export type FetchState = {
  fetchUsersOptions(input: string): Promise<HybridUserOption[]>;
  fetchAccessListsOptions(input: string): Promise<HybridUserOption[]>;
};

export function useFetch(): FetchState {
  const ctx = useTeleport();

  const fetchUsersOptions = useCallback(
    async (input: string) => {
      const usersResult = await ctx.userService.fetchUsersV2({
        search: input,
        limit: 50,
        searchMode: 'identity',
      });

      const options: HybridUserOption[] = usersResult.items.map(user => ({
        label: user.name,
        value: {
          membershipKind: AccessListMemberKind.User,
          name: user.name,
          displayPrimary: user.displayPrimary,
          displaySecondary: user.displaySecondary,
        },
      }));

      return options;
    },
    [ctx.userService]
  );

  const fetchAccessListsOptions = useCallback(async (input: string) => {
    const accessListsResult = await accessManagementService.fetchAccessListsV2({
      // sort by name here. If they want to find a specific list to add, they will most
      // likely type, but this allows this form to work without extra steps to check
      // if the cache is healthy or not
      sort: { dir: 'ASC', fieldName: 'name' },
      search: input,
      limit: 50,
    });

    const options: HybridUserOption[] = accessListsResult.agents.map(list => ({
      label: list.title,
      value: {
        membershipKind: AccessListMemberKind.List,
        name: list.id,
        origin: list.origin,
      },
    }));

    return options;
  }, []);

  return {
    fetchUsersOptions,
    fetchAccessListsOptions,
  };
}
