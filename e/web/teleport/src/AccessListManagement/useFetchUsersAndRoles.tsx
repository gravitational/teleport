import { useState } from 'react';
import { State as AttemptState } from 'shared/hooks/useAttemptNext';
import { Option } from 'shared/components/Select';
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
  const [roleOptions, setRoleOptions] = useState<Option[]>([]);

  const { setAttempt } = attempt;

  function fetchUsersAndRoles() {
    const resourceSvc = new ResourceService();
    const userAccess = ctx.storeUser.getUserAccess();
    const roleAccess = ctx.storeUser.getRoleAccess();

    const promises = [];

    if (userAccess.list && userAccess.read) {
      promises.push(
        userService.fetchUsers().then(fetchedUsers => {
          setUserOptions(fetchedUsers.map(u => ({ value: u, label: u.name })));
        })
      );
    }

    if (roleAccess.list && roleAccess.read) {
      promises.push(
        resourceSvc.fetchRoles().then(fetchedRoles => {
          setRoleOptions(
            fetchedRoles.map(u => ({ value: u.name, label: u.name }))
          );
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

  return {
    fetchUsersAndRoles,
    userOptions,
    roleOptions,
  };
}
