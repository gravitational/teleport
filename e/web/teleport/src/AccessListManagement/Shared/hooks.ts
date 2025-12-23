import { useCallback, useMemo } from 'react';

import { debounce } from 'shared/utils/highbar';

import type { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

/**
 * useUserOptions returns a users option loader suitable to be used
 * in a user select component. It loads a default page of 50 items
 * and then subsequent page is requested based on the search input.
 * @param collector collects option labels and values.
 * @returns user options loader.
 */
export function useUserOptions<T>(collector: (user: User[]) => T[]) {
  const ctx = useTeleport();

  const fetchUsersOptions = useCallback(
    async (input: string) => {
      const usersResult = await ctx.userService.fetchUsersV2({
        search: input,
        limit: 50,
      });

      return collector(usersResult.items);
    },
    [ctx.userService]
  );

  const debouncedFn = useMemo(
    () =>
      debounce(
        async (
          searchInput: string,
          resolve: (result: T[]) => void,
          reject: (error: unknown) => void
        ) => {
          try {
            const result = await fetchUsersOptions(searchInput);
            resolve(result);
          } catch (e) {
            reject(e);
          }
        },
        300
      ),
    [fetchUsersOptions]
  );

  const loadOptions = useCallback(
    async (input: string): Promise<T[]> => {
      if (!input) {
        return fetchUsersOptions('');
      }
      return new Promise((resolve, reject) => {
        debouncedFn(input, resolve, reject);
      });
    },
    [debouncedFn, fetchUsersOptions]
  );

  return {
    loadOptions,
  };
}
