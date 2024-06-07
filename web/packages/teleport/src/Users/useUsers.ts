/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { ReactElement, useEffect, useState } from 'react';
import { useAttempt } from 'shared/hooks';

import { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';
import auth from 'teleport/services/auth/auth';

export default function useUsers({
  InviteCollaborators,
  EmailPasswordReset,
}: UsersContainerProps) {
  const ctx = useTeleport();
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [users, setUsers] = useState([] as User[]);
  const [operation, setOperation] = useState({
    type: 'none',
  } as Operation);
  const [inviteCollaboratorsOpen, setInviteCollaboratorsOpen] =
    useState<boolean>(false);

  function onStartCreate() {
    const user = { name: '', roles: [], created: new Date() };
    setOperation({
      type: 'create',
      user,
    });
  }

  function onStartEdit(user: User) {
    setOperation({ type: 'edit', user });
  }

  function onStartDelete(user: User) {
    setOperation({ type: 'delete', user });
  }

  function onStartReset(user: User) {
    setOperation({ type: 'reset', user });
  }

  function onStartInviteCollaborators(user: User) {
    setOperation({ type: 'invite-collaborators', user });
    setInviteCollaboratorsOpen(true);
  }

  function onClose() {
    setOperation({ type: 'none' });
  }

  function onReset(name: string) {
    return ctx.userService.createResetPasswordToken(name, 'password');
  }

  function onDelete(name: string) {
    return ctx.userService.deleteUser(name).then(() => {
      const updatedUsers = users.filter(user => user.name !== name);
      setUsers(updatedUsers);
    });
  }

  function onUpdate(u: User) {
    return ctx.userService.updateUser(u).then(result => {
      setUsers([result, ...users.filter(i => i.name !== u.name)]);
    });
  }

  async function onCreate(u: User) {
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(true);
    return ctx.userService
      .createUser(u, webauthnResponse)
      .then(result => setUsers([result, ...users]))
      .then(() =>
        ctx.userService.createResetPasswordToken(
          u.name,
          'invite',
          webauthnResponse
        )
      );
  }

  function onInviteCollaboratorsClose(newUsers?: User[]) {
    if (newUsers && newUsers.length > 0) {
      setUsers([...newUsers, ...users]);
    }

    setInviteCollaboratorsOpen(false);
    setOperation({ type: 'none' });
  }

  function onEmailPasswordResetClose() {
    setOperation({ type: 'none' });
  }

  async function fetchRoles(search: string): Promise<string[]> {
    const { items } = await ctx.resourceService.fetchRoles({
      search,
      limit: 50,
    });
    return items.map(r => r.name);
  }

  useEffect(() => {
    attemptActions.do(() => ctx.userService.fetchUsers().then(setUsers));
  }, []);

  return {
    attempt,
    users,
    fetchRoles,
    operation,
    onStartCreate,
    onStartDelete,
    onStartEdit,
    onStartReset,
    onStartInviteCollaborators,
    onClose,
    onDelete,
    onCreate,
    onUpdate,
    onReset,
    onInviteCollaboratorsClose,
    InviteCollaborators,
    inviteCollaboratorsOpen,
    onEmailPasswordResetClose,
    EmailPasswordReset,
  };
}

type Operation = {
  type:
    | 'create'
    | 'invite-collaborators'
    | 'edit'
    | 'delete'
    | 'reset'
    | 'none';
  user?: User;
};

export interface InviteCollaboratorsDialogProps {
  onClose: (users?: User[]) => void;
  open: boolean;
}

export interface EmailPasswordResetDialogProps {
  username: string;
  onClose: () => void;
}

type InviteCollaboratorsElement = (
  props: InviteCollaboratorsDialogProps
) => ReactElement;
type EmailPasswordResetElement = (
  props: EmailPasswordResetDialogProps
) => ReactElement;

export type UsersContainerProps = {
  InviteCollaborators?: InviteCollaboratorsElement;
  EmailPasswordReset?: EmailPasswordResetElement;
};

export type State = ReturnType<typeof useUsers>;
