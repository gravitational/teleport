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

export default function Container(props: UsersContainerProps) {
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
