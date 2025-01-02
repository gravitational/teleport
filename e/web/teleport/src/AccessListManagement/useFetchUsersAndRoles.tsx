import { useState } from 'react';

import { Option } from 'shared/components/Select';
import { State as AttemptState } from 'shared/hooks/useAttemptNext';

import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import { UserOption } from './Shared/Shared';

// Fetching list of users and roles is a nice to have for
// users who have perms to do so because it will generate
// dropdown options for existing users and roles when editing
// and creating list of members and owners.
//
// Otherwise, no permission means fetching will be skipped and
// the user will have to manually input user and role names
// when editing/creating.
export function useFetchUserAndRoles(attempt: AttemptState) {
  const ctx = useTeleport();

  const [userOptions, setUserOptions] = useState<UserOption[]>([]);

  const { setAttempt } = attempt;

  function fetchUsersAndRoles() {
    const userAccess = ctx.storeUser.getUserAccess();

    const promises = [];

    if (userAccess.list && userAccess.read) {
      promises.push(
        userService.fetchUsers().then(fetchedUsers => {
          setUserOptions(fetchedUsers.map(u => ({ value: u, label: u.name })));
        })
      );
    }

    Promise.all(promises)
      .then(() => {
        setAttempt({ status: 'success' });
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  async function fetchRoleOptions(search: string): Promise<Option[]> {
    const roleAccess = ctx.storeUser.getRoleAccess();

    if (roleAccess.list && roleAccess.read) {
      const resourceSvc = new ResourceService();
      const roles = await resourceSvc.fetchRoles({ search, limit: 50 });
      return roles.items.map(r => ({ value: r.name, label: r.name }));
    }

    return [];
  }

  return {
    fetchUsersAndRoles,
    userOptions,
    fetchRoleOptions,
  };
}
