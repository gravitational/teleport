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

import { Alert, Box, Button, Flex, Indicator, Link, Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import UserAddEdit from './UserAddEdit';
import UserDelete from './UserDelete';
import UserList from './UserList';
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
    usersAcl,
    showMauInfo,
    onDismissUsersMauNotice,
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

  const requiredPermissions = Object.entries(usersAcl)
    .map(([key, value]) => {
      if (key === 'edit') {
        return { value, label: 'update' };
      }
      if (key === 'create') {
        return { value, label: 'create' };
      }
    })
    .filter(Boolean);

  const isMissingPermissions = requiredPermissions.some(v => !v.value);

  return (
    <FeatureBox>
      <FeatureHeader justifyContent="space-between">
        <FeatureHeaderTitle>Users</FeatureHeaderTitle>
        {attempt.isSuccess && (
          <>
            {!InviteCollaborators && (
              <HoverTooltip
                position="bottom"
                tipContent={
                  !isMissingPermissions ? (
                    ''
                  ) : (
                    <Box>
                      {/* TODO (avatus): extract this into a new "missing permissions" component. This will
                          require us to change the internals of HoverTooltip to allow more arbitrary styling of the popover.
                      */}
                      <Text mb={1}>
                        You do not have all of the required permissions.
                      </Text>
                      <Box mb={1}>
                        <Text bold>You are missing permissions:</Text>
                        <Flex gap={2}>
                          {requiredPermissions
                            .filter(perm => !perm.value)
                            .map(perm => (
                              <Text
                                key={perm.label}
                              >{`users.${perm.label}`}</Text>
                            ))}
                        </Flex>
                      </Box>
                    </Box>
                  )
                }
              >
                <Button
                  intent="primary"
                  data-testid="create_new_users_button"
                  fill="border"
                  disabled={!usersAcl.edit}
                  ml="auto"
                  width="240px"
                  onClick={onStartCreate}
                >
                  Create New User
                </Button>
              </HoverTooltip>
            )}
            {InviteCollaborators && (
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
      {attempt.isProcessing && (
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
      {attempt.isFailed && <Alert kind="danger" children={attempt.message} />}
      {attempt.isSuccess && (
        <UserList
          usersAcl={usersAcl}
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
