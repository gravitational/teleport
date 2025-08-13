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

import { ReactElement, useState } from 'react';

import cfg, { UrlListUsersParams } from 'teleport/config';
import { storageService } from 'teleport/services/storageService';
import { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

export default function useUsers({
  InviteCollaborators,
  EmailPasswordReset,
}: UsersContainerProps) {
  const ctx = useTeleport();
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

  function onInviteCollaboratorsClose() {
    setInviteCollaboratorsOpen(false);
    setOperation({ type: 'none' });
  }

  function onEmailPasswordResetClose() {
    setOperation({ type: 'none' });
  }

  function onDismissUsersMauNotice() {
    storageService.setUsersMAUAcknowledged();
  }

  // if the cluster has billing enabled, and usageBasedBilling, and they haven't acknowledged
  // the info yet
  const showMauInfo =
    ctx.getFeatureFlags().billing &&
    cfg.isUsageBasedBilling &&
    !storageService.getUsersMauAcknowledged();

  const usersAcl = ctx.storeUser.getUserAccess();

  function fetch(params?: UrlListUsersParams, signal?: AbortSignal) {
    return ctx.userService.fetchUsersV2(params, signal);
  }

  return {
    usersAcl,
    operation,
    onStartCreate,
    onStartDelete,
    onStartEdit,
    onStartReset,
    onStartInviteCollaborators,
    onClose,
    onReset,
    onInviteCollaboratorsClose,
    InviteCollaborators,
    inviteCollaboratorsOpen,
    onEmailPasswordResetClose,
    EmailPasswordReset,
    showMauInfo,
    onDismissUsersMauNotice,
    fetch,
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
