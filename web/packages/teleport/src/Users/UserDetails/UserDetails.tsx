/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Label, Text } from 'design';
import * as Icons from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import {
  InfoParagraph,
  InfoTitle,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { useListLocks } from 'teleport/services/locks';
import { User } from 'teleport/services/user';

import { getUserAuthType, UserAuthType } from './types';
import { UserRolesModal } from './UserRoles';

function useUserLockStatus(username: string) {
  const { data: locks } = useListLocks({
    inForceOnly: true,
    targets: [{ kind: 'user', name: username }],
  });

  return locks?.length > 0;
}

function renderUserStatus(isLocked: boolean) {
  if (isLocked) {
    return {
      text: 'Locked',
      dotColor: 'error.main',
    };
  }
  return {
    text: 'Active',
    dotColor: 'success.main',
  };
}

function renderAuthType(user: User) {
  if (user.isBot) return { text: 'Bot', icon: <Icons.Bots size="small" /> };

  switch (user.authType) {
    case 'github':
      return { text: 'GitHub', icon: <Icons.GitHub size="small" /> };
    case 'saml':
      switch (user.origin) {
        case 'okta':
          return {
            text: 'Okta',
            icon: <ResourceIcon name="okta" width="16px" height="16px" />,
          };
        case 'scim':
          return {
            text: 'SCIM',
            icon: <ResourceIcon name="scim" width="16px" height="16px" />,
          };
        default:
          return { text: 'SAML', icon: <Icons.ShieldCheck size="small" /> };
      }
    case 'oidc':
      return { text: 'OIDC', icon: <Icons.ShieldCheck size="small" /> };
    default:
      return {
        text: user.authType || 'Local',
        icon: <Icons.Key size="small" />,
      };
  }
}

export function UserDetailsTitle({ user }: { user: User }) {
  return (
    <Flex alignItems="flex-start" gap={3} mt={2} mb={2}>
      <Icons.UserIdBadge size="extra-large" />
      <Box>
        <Text fontSize={4} fontWeight="bold" mb={1}>
          {user.name}
        </Text>
        <Flex alignItems="center" gap={2}>
          {renderAuthType(user).icon}
          <Text fontSize={2} color="text.muted">
            {renderAuthType(user).text}
          </Text>
        </Flex>
      </Box>
    </Flex>
  );
}

export interface UserDetailsSectionProps {
  user: User;
}

export interface UserDetailsProps {
  user: User;
  sections?: React.ComponentType<UserDetailsSectionProps>[];
}

export function UserRoles({ user }: UserDetailsSectionProps) {
  const [isRolesModalOpen, setIsRolesModalOpen] = useState(false);

  if (!user?.roles) return null;

  return (
    <>
      <InfoTitle>Roles ({user.roles?.length || 0})</InfoTitle>
      <InfoParagraph>
        {user.roles && user.roles.length > 0 && (
          <Flex gap={2} flexWrap="wrap">
            {user.roles.slice(0, 7).map(role => (
              <Label key={role} kind="secondary">
                <Flex gap={1}>
                  <Icons.UserIdBadge size={16} />
                  {role}
                </Flex>
              </Label>
            ))}
            {user.roles.length > 7 && (
              <ClickableLabel
                kind="secondary"
                onClick={() => setIsRolesModalOpen(true)}
              >
                + {user.roles.length - 7} more
              </ClickableLabel>
            )}
          </Flex>
        )}
      </InfoParagraph>

      <UserRolesModal
        isOpen={isRolesModalOpen}
        onClose={() => setIsRolesModalOpen(false)}
        roles={user.roles || []}
        userName={user.name}
      />
    </>
  );
}

export function UserTraits({ user }: UserDetailsSectionProps) {
  if (!user?.allTraits) return null;

  return (
    <>
      <InfoTitle>Traits</InfoTitle>
      <InfoParagraph>
        <Flex flexWrap="wrap" gap={2}>
          {Object.keys(user.allTraits)
            .filter(key => user.allTraits[key].length > 0)
            .map(key => (
              <Label key={key} kind="secondary">
                {key}: {user.allTraits[key].join(', ')}
              </Label>
            ))}
        </Flex>
      </InfoParagraph>
    </>
  );
}

export function UserDetails({ user, sections = [] }: UserDetailsProps) {
  const isLocked = useUserLockStatus(user?.name || '');

  if (!user) return null;

  return (
    <Box>
      <InfoParagraph>
        <InfoTitle>User details</InfoTitle>
        <UserDetailsGrid>
          <UserDetailField>
            <Text fontWeight="medium">Username</Text>
            <Text color="text.muted">{user.name}</Text>
          </UserDetailField>
          <UserDetailField>
            <Text fontWeight="medium">Auth Type</Text>
            <Text color="text.muted">{renderAuthType(user).text}</Text>
          </UserDetailField>
          <UserDetailField>
            <Text fontWeight="medium">Status</Text>
            <Flex alignItems="center" gap={2}>
              <Box
                width="8px"
                height="8px"
                borderRadius="50%"
                backgroundColor={isLocked ? 'error.main' : 'success.main'}
              />
              <Text color="text.muted">{isLocked ? 'Locked' : 'Active'}</Text>
            </Flex>
          </UserDetailField>
        </UserDetailsGrid>
      </InfoParagraph>

      {sections.map((SectionComponent, index) => (
        <UserDetailsSection key={index}>
          <SectionComponent user={user} />
        </UserDetailsSection>
      ))}
    </Box>
  );
}

export const UserDetailsSection = styled(Box)`
  border-bottom: 1px solid ${props => props.theme.colors.levels.sunken};
  margin: -${props => props.theme.space[3]}px;
  padding: ${props => props.theme.space[3]}px;

  &:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }
`;

export const UserDetailsGrid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: ${props => props.theme.space[3]}px;
`;

export const UserDetailField = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${props => props.theme.space[1]}px;
`;

export const StyledInfoParagraph = styled(InfoParagraph)`
  border-bottom: 1px solid ${props => props.theme.colors.levels.sunken};

  margin: -${props => props.theme.space[3]}px;
  padding: ${props => props.theme.space[3]}px;

  &:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }
`;

const ClickableLabel = styled(Label)`
  cursor: pointer;

  &:hover {
    background: ${props => props.theme.colors.levels.elevated};
  }
`;
