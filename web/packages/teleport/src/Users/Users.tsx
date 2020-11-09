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
import useTeleport from 'teleport/useTeleport';
import UserList from './UserList';
import UserAddEdit from './UserAddEdit';
import UserDelete from './UserDelete';
import UserReset from './UserReset';
import useUsers from './useUsers';

export default function Container() {
  const state = useUsers();
  return <Users {...state} />;
}

export function Users(props: ReturnType<typeof useUsers>) {
  const {
    attempt,
    users,
    roles,
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
  } = props;

  const ctx = useTeleport().storeUser;
  const canCreate = attempt.isSuccess && ctx.getUserAccess().create;
  const canDelete = ctx.getUserAccess().remove;
  const canUpdate = ctx.getUserAccess().edit;

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Users</FeatureHeaderTitle>
        {canCreate && (
          <ButtonPrimary ml="auto" width="240px" onClick={onStartCreate}>
            Create New User
          </ButtonPrimary>
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
          pageSize={20}
          onEdit={onStartEdit}
          onDelete={onStartDelete}
          onReset={onStartReset}
          canDelete={canDelete}
          canUpdate={canUpdate}
        />
      )}
      {(operation.type === 'create' || operation.type === 'edit') && (
        <UserAddEdit
          isNew={operation.type === 'create'}
          roles={roles}
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
      {operation.type === 'reset' && (
        <UserReset
          onClose={onClose}
          onReset={onReset}
          username={operation.user.name}
        />
      )}
    </FeatureBox>
  );
}
