/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ReactElement, useState, useEffect } from 'react';
import { useAttempt } from 'shared/hooks';

import { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

export default function useUsers({ inviteCollaborators }: UsersContainerProps) {
  const ctx = useTeleport();
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [users, setUsers] = useState([] as User[]);
  const [roles, setRoles] = useState([] as string[]);
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

  function onCreate(u: User) {
    return ctx.userService
      .createUser(u)
      .then(result => setUsers([result, ...users]))
      .then(() => ctx.userService.createResetPasswordToken(u.name, 'invite'));
  }

  function onInviteCollaboratorsClose(newUsers?: User[]) {
    if (newUsers && newUsers.length > 0) {
      setUsers([...newUsers, ...users]);
    }

    setInviteCollaboratorsOpen(false);
    setOperation({ type: 'none' });
  }

  useEffect(() => {
    function fetchRoles() {
      if (ctx.getFeatureFlags().roles) {
        return ctx.resourceService
          .fetchRoles()
          .then(resources => resources.map(role => role.name));
      }

      return Promise.resolve([]);
    }

    attemptActions.do(() =>
      Promise.all([fetchRoles(), ctx.userService.fetchUsers()]).then(values => {
        setRoles(values[0]);
        setUsers(values[1]);
      })
    );
  }, []);

  return {
    attempt,
    users,
    roles,
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
    inviteCollaborators,
    inviteCollaboratorsOpen,
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

export type UsersContainerProps = {
  inviteCollaborators?: (props: InviteCollaboratorsDialogProps) => ReactElement;
};

export type State = ReturnType<typeof useUsers>;
