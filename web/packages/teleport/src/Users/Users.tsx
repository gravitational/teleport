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

import React, { useEffect, useRef, useState } from 'react';
import { Link as InternalLink } from 'react-router-dom';

import { Alert, Box, Button, Link as ExternalLink, Flex, Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import {
  InfoExternalTextLink,
  InfoGuideButton,
  InfoParagraph,
  InfoUl,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { useServerSidePagination } from 'teleport/components/hooks';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { User } from 'teleport/services/user';

import { UserAddEdit } from './UserAddEdit';
import { UserDelete } from './UserDelete';
import UserList from './UserList';
import UserReset from './UserReset';
import useUsers, { State, UsersContainerProps } from './useUsers';

export function UsersContainer(props: UsersContainerProps) {
  const state = useUsers(props);
  return <Users {...state} />;
}

export function Users(props: State) {
  const {
    operation,
    onStartCreate,
    onStartDelete,
    onStartEdit,
    onStartReset,
    usersAcl,
    showMauInfo,
    onDismissUsersMauNotice,
    onClose,
    onReset,
    onStartInviteCollaborators,
    onInviteCollaboratorsClose,
    inviteCollaboratorsOpen,
    InviteCollaborators,
    EmailPasswordReset,
    onEmailPasswordResetClose,
    fetch,
  } = props;

  const [search, setSearch] = useState('');
  const abortControllerRef = useRef<AbortController | null>(null);

  const serverSidePagination = useServerSidePagination<User>({
    pageSize: 20,
    fetchFunc: async (_, params) => {
      const { items, startKey } = await fetch(
        params,
        abortControllerRef.current?.signal
      );
      return { agents: items || [], startKey };
    },
    clusterId: '',
    params: { search },
  });

  useEffect(() => {
    // Cancel previous request and create new controller
    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();

    serverSidePagination.fetch();
  }, [search]);

  // Cleanup controller on unmount
  useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

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
        {serverSidePagination.attempt.status === 'success' && (
          <Flex gap={2}>
            {!InviteCollaborators && (
              <HoverTooltip
                placement="bottom"
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
            <InfoGuideButton config={{ guide: <InfoGuide /> }} />
          </Flex>
        )}
      </FeatureHeader>
      {serverSidePagination.attempt.status === 'failed' && (
        <Alert>{serverSidePagination.attempt.statusText}</Alert>
      )}
      {showMauInfo && serverSidePagination.attempt.status !== 'processing' && (
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
          <ExternalLink
            target="_blank"
            href="https://goteleport.com/docs/usage-billing/#monthly-active-users"
            className="external-link"
          >
            MAU
          </ExternalLink>{' '}
          and{' '}
          <ExternalLink
            href="https://goteleport.com/docs/reference/user-types/"
            className="external-link"
          >
            User Types
          </ExternalLink>
          .
        </Alert>
      )}
      <UserList
        serversidePagination={serverSidePagination}
        onSearchChange={setSearch}
        search={search}
        onEdit={onStartEdit}
        onDelete={onStartDelete}
        onReset={onStartReset}
        usersAcl={usersAcl}
      />
      {(operation.type === 'create' || operation.type === 'edit') && (
        <UserAddEdit
          isNew={operation.type === 'create'}
          onClose={onClose}
          user={operation.user}
          modifyFetchedData={serverSidePagination.modifyFetchedData}
        />
      )}
      {operation.type === 'delete' && (
        <UserDelete
          onClose={onClose}
          username={operation.user.name}
          modifyFetchedData={serverSidePagination.modifyFetchedData}
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
      {InviteCollaborators && inviteCollaboratorsOpen && (
        <InviteCollaborators onClose={onInviteCollaboratorsClose} />
      )}
    </FeatureBox>
  );
}

const InfoGuideReferenceLinks = {
  Users: {
    title: 'Teleport Users',
    href: 'https://goteleport.com/docs/core-concepts/#teleport-users',
  },
};

const InfoGuide = () => (
  <Box>
    <InfoParagraph>
      Teleport allows for two kinds of{' '}
      <InfoExternalTextLink href={InfoGuideReferenceLinks.Users.href}>
        users
      </InfoExternalTextLink>
      :
      <InfoUl>
        <li>
          <b>Local</b> users are created and managed in Teleport and stored in
          the Auth Service backend.
        </li>
        <li>
          <b>Single Sign-On (SSO)</b> users are stored on the backend of your
          SSO solution, e.g., Okta or GitHub. SSO can be set up with an{' '}
          <InternalLink to={cfg.routes.sso}>Auth Connector</InternalLink>.
        </li>
      </InfoUl>
    </InfoParagraph>
    <InfoParagraph>
      To take any action in Teleport, users must have at least one{' '}
      <InternalLink to={cfg.routes.roles}>Role</InternalLink> assigned.
    </InfoParagraph>
    <ReferenceLinks links={Object.values(InfoGuideReferenceLinks)} />
  </Box>
);
