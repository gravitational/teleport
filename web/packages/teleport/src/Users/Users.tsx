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

import React, { ComponentType, useCallback, useState } from 'react';
import { Alert, Box, Button, Indicator, Link } from 'design';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useTeleport } from 'teleport';
import { ExcludeUserField, User } from 'teleport/services/user';
import { storageService } from 'teleport/services/storageService';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth';

import UserReset from './UserReset';
import UserDelete from './UserDelete';
import UserAddEdit from './UserAddEdit';
import UserList from './UserList';

interface InviteCollaboratorsProps {
  onClose: (users?: User[]) => void;
  open: boolean;
}

interface EmailPasswordResetProps {
  onClose: () => void;
  username: string;
}

interface UsersProps {
  inviteCollaboratorsComponent?: ComponentType<InviteCollaboratorsProps>;
  emailPasswordResetComponent?: ComponentType<EmailPasswordResetProps>;
}

enum OperationType {
  None,
  Create,
  InviteCollaborators,
  Edit,
  Delete,
  Reset,
}

interface OperationWithUser {
  type:
    | OperationType.Create
    | OperationType.Edit
    | OperationType.Delete
    | OperationType.Reset
    | OperationType.InviteCollaborators;
  user: User;
}

interface OperationWithoutUser {
  type: OperationType.None;
}

type Operation = OperationWithUser | OperationWithoutUser;

export function Users(props: UsersProps) {
  const ctx = useTeleport();

  const queryClient = useQueryClient();

  const [operation, setOperation] = useState<Operation>({
    type: OperationType.None,
  });

  const [inviteCollaboratorsOpen, setInviteCollaboratorsOpen] = useState(false);

  const { error, data, isPending, isError, isSuccess } = useQuery<
    User[],
    Error
  >({
    queryKey: ['users'],
    queryFn: ({ signal }) => ctx.userService.fetchUsers(signal),
  });

  const onStartCreate = useCallback(() => {
    setOperation({ type: OperationType.Create, user: { name: '', roles: [] } });
  }, []);

  const onStartEdit = useCallback((user: User) => {
    setOperation({ type: OperationType.Edit, user });
  }, []);

  const onStartDelete = useCallback((user: User) => {
    setOperation({ type: OperationType.Delete, user });
  }, []);

  const onStartReset = useCallback((user: User) => {
    setOperation({ type: OperationType.Reset, user });
  }, []);

  const onStartInviteCollaborators = useCallback((user: User) => {
    setOperation({ type: OperationType.InviteCollaborators, user });
    setInviteCollaboratorsOpen(true);
  }, []);

  const onClose = useCallback(() => {
    setOperation({ type: OperationType.None });
  }, []);

  const onDismissUsersMauNotice = useCallback(() => {
    storageService.setUsersMAUAcknowledged();
  }, []);

  const onInviteCollaboratorsClose = useCallback(
    (newUsers?: User[]) => {
      if (newUsers && newUsers.length > 0) {
        queryClient.setQueryData(['users'], (users: User[]) => [
          ...newUsers,
          ...users,
        ]);
      }

      setInviteCollaboratorsOpen(false);
      setOperation({ type: OperationType.None });
    },
    [queryClient]
  );

  const showMauInfo =
    ctx.getFeatureFlags().billing &&
    cfg.isUsageBasedBilling &&
    !storageService.getUsersMauAcknowledged();

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Users</FeatureHeaderTitle>
        {isSuccess && (
          <>
            {!props.inviteCollaboratorsComponent && (
              <Button
                intent="primary"
                fill="border"
                ml="auto"
                width="240px"
                onClick={onStartCreate}
              >
                Create New User
              </Button>
            )}
            {props.inviteCollaboratorsComponent && (
              <Button
                intent="primary"
                fill="border"
                ml="auto"
                width="240px"
                // TODO(bl-nero): There may be a bug here that used to be hidden
                // by inadequate type checking; investigate and fix.
                onClick={
                  onStartInviteCollaborators as any as React.MouseEventHandler<HTMLButtonElement>
                }
              >
                Enroll Users
              </Button>
            )}
          </>
        )}
      </FeatureHeader>
      {isPending && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {showMauInfo && (
        <Alert
          data-testid="users-not-mau-alert"
          dismissible
          onDismiss={onDismissUsersMauNotice}
          kind="info"
          css={`
            a.external-link {
              color: ${({ theme }) => theme.colors.buttons.link.default};
            }
          `}
        >
          The users displayed here are not an accurate reflection of Monthly
          Active Users (MAU). For example, users who log in through Single
          Sign-On (SSO) providers such as Okta may only appear here temporarily
          and disappear once their sessions expire. For more information, read
          our documentation on{' '}
          <Link
            target="_blank"
            href="https://goteleport.com/docs/usage-billing/#monthly-active-users"
            className="external-link"
          >
            MAU
          </Link>{' '}
          and{' '}
          <Link
            href="https://goteleport.com/docs/reference/user-types/"
            className="external-link"
          >
            User Types
          </Link>
          .
        </Alert>
      )}
      {isError && <Alert kind="danger" children={error.message} />}
      {isSuccess && (
        <UserList
          users={data}
          onEdit={onStartEdit}
          onDelete={onStartDelete}
          onReset={onStartReset}
        />
      )}
      {operation.type !== OperationType.None && (
        <UserOperations
          emailPasswordResetComponent={props.emailPasswordResetComponent}
          operation={operation}
          onClose={onClose}
        />
      )}
      {props.inviteCollaboratorsComponent && (
        <props.inviteCollaboratorsComponent
          open={inviteCollaboratorsOpen}
          onClose={onInviteCollaboratorsClose}
        />
      )}
    </FeatureBox>
  );
}

interface UserOperationsProps {
  operation: OperationWithUser;
  onClose: () => void;
  emailPasswordResetComponent: ComponentType<EmailPasswordResetProps> | null;
}

function UserOperations(props: UserOperationsProps) {
  if (
    props.operation.type === OperationType.Create ||
    props.operation.type === OperationType.Edit
  ) {
    return <CreateOrEditUser {...props} />;
  }

  if (props.operation.type === OperationType.Delete) {
    return <DeleteUser {...props} />;
  }

  if (props.operation.type === OperationType.Reset) {
    if (props.emailPasswordResetComponent) {
      return (
        <props.emailPasswordResetComponent
          onClose={props.onClose}
          username={props.operation.user.name}
        />
      );
    }

    return <ResetUser {...props} />;
  }

  return null;
}

function CreateOrEditUser(props: UserOperationsProps) {
  const ctx = useTeleport();

  const queryClient = useQueryClient();

  const create = useMutation({
    mutationFn: async (user: User) => {
      const webauthnResponse =
        await auth.getWebauthnResponseForAdminAction(true);

      const createdUser = await ctx.userService.createUser(
        user,
        ExcludeUserField.Traits,
        webauthnResponse
      );

      const token = ctx.userService.createResetPasswordToken(
        user.name,
        'invite',
        webauthnResponse
      );

      return { user: createdUser, token };
    },
    onSuccess({ user }) {
      queryClient.setQueryData(['users'], (users: User[]) => [user, ...users]);
    },
  });

  const update = useMutation({
    mutationFn: (user: User) =>
      ctx.userService.updateUser(user, ExcludeUserField.Traits),
    onSuccess(user) {
      queryClient.setQueryData(['users'], (users: User[]) => [
        user,
        ...users.filter(u => u.name !== user.name),
      ]);
    },
  });

  const onCreate = useCallback(
    async (user: User) => {
      const { token } = await create.mutateAsync(user);

      return token;
    },
    [create]
  );
  const onUpdate = useCallback(
    (user: User) => update.mutateAsync(user),
    [update]
  );

  return (
    <UserAddEdit
      isNew={props.operation.type === OperationType.Create}
      onClose={props.onClose}
      onCreate={onCreate}
      onUpdate={onUpdate}
      user={props.operation.user}
    />
  );
}

function DeleteUser(props: UserOperationsProps) {
  const ctx = useTeleport();

  const queryClient = useQueryClient();

  const deleteUser = useMutation({
    mutationFn: (name: string) => ctx.userService.deleteUser(name),
    onSuccess(_, name) {
      queryClient.setQueryData(['users'], (users: User[]) =>
        users.filter(u => u.name !== name)
      );
    },
  });

  const onDelete = useCallback(
    (name: string) => deleteUser.mutateAsync(name),
    [deleteUser]
  );

  return (
    <UserDelete
      onClose={props.onClose}
      onDelete={onDelete}
      username={props.operation.user.name}
    />
  );
}

function ResetUser(props: UserOperationsProps) {
  const ctx = useTeleport();

  const resetPassword = useMutation({
    mutationFn: (name: string) =>
      ctx.userService.createResetPasswordToken(name, 'password'),
  });

  const onReset = useCallback(
    (name: string) => resetPassword.mutateAsync(name),
    [resetPassword]
  );

  return (
    <UserReset
      onClose={props.onClose}
      onReset={onReset}
      username={props.operation.user.name}
    />
  );
}
