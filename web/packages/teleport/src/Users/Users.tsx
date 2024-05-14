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

import React from 'react';
import { Indicator, Box, ButtonPrimary, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import UserList from './UserList';
import UserAddEdit from './UserAddEdit';
import UserDelete from './UserDelete';
import UserReset from './UserReset';
import useUsers, { State, UsersContainerProps } from './useUsers';

export function UsersContainer(props: UsersContainerProps) {
  const state = useUsers(props);
  return <Users {...state} />;
}

export function Users(props: State) {
  const {
    attempt,
    users,
    fetchRoles,
    operation,
    onStartCreate,
    onStartDelete,
    onStartEdit,
    onStartReset,
    onClose,
    onCreate,
    onUpdate,
    onDelete,
    onReset,
    onStartInviteCollaborators,
    onInviteCollaboratorsClose,
    inviteCollaboratorsOpen,
    InviteCollaborators,
    EmailPasswordReset,
    onEmailPasswordResetClose,
  } = props;
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Users</FeatureHeaderTitle>
        {attempt.isSuccess && (
          <>
            {!InviteCollaborators && (
              <ButtonPrimary ml="auto" width="240px" onClick={onStartCreate}>
                Create New User
              </ButtonPrimary>
            )}
            {InviteCollaborators && (
              <ButtonPrimary
                ml="auto"
                width="240px"
                onClick={onStartInviteCollaborators}
              >
                Enroll Users
              </ButtonPrimary>
            )}
          </>
        )}
      </FeatureHeader>
      {attempt.isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.isFailed && <Alert kind="danger" children={attempt.message} />}
      {attempt.isSuccess && (
        <UserList
          users={users}
          onEdit={onStartEdit}
          onDelete={onStartDelete}
          onReset={onStartReset}
        />
      )}
      {(operation.type === 'create' || operation.type === 'edit') && (
        <UserAddEdit
          isNew={operation.type === 'create'}
          fetchRoles={fetchRoles}
          onClose={onClose}
          onCreate={onCreate}
          onUpdate={onUpdate}
          user={operation.user}
        />
      )}
      {operation.type === 'delete' && (
        <UserDelete
          onClose={onClose}
          onDelete={onDelete}
          username={operation.user.name}
        />
      )}
      {operation.type === 'reset' && !EmailPasswordReset && (
        <UserReset
          onClose={onClose}
          onReset={onReset}
          username={operation.user.name}
        />
      )}
      {operation.type === 'reset' && EmailPasswordReset && (
        <EmailPasswordReset
          onClose={onEmailPasswordResetClose}
          username={operation.user.name}
        />
      )}
      {InviteCollaborators && (
        <InviteCollaborators
          open={inviteCollaboratorsOpen}
          onClose={onInviteCollaboratorsClose}
        />
      )}
    </FeatureBox>
  );
}
