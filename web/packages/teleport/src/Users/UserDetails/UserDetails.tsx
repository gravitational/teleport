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

import { useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, H3, H4, Label, Pill, Text } from 'design';
import { Icon } from 'design/Icon';
import * as Icons from 'design/Icon';
import { SlidingSidePanel } from 'shared/components/SlidingSidePanel';

import { User } from 'teleport/services/user';
import { useGetUser } from 'teleport/services/user/hooks';

import { UserRoles } from './UserRoles';

export interface UserDetailsProps {
  isVisible: boolean;
  onClose: () => void;
  user: User | null;
}

export function UserDetails({ isVisible, onClose, user }: UserDetailsProps) {
  const [isRolesModalOpen, setIsRolesModalOpen] = useState(false);
  const { data: userDetails, isLoading } = useGetUser(user?.name, {
    enabled: !!user?.name,
  });
  const renderAuthType = (user: User) => {
    if (user.isBot) return { text: 'Bot', icon: Icons.Bots };

    switch (user.authType) {
      case 'github':
        return { text: 'GitHub', icon: Icons.GitHub };
      case 'saml':
        switch (user.origin) {
          case 'okta':
            return { text: 'Okta', icon: Icons.ShieldCheck };
          case 'scim':
            return { text: 'SCIM', icon: Icons.ShieldCheck };
          default:
            return { text: 'SAML', icon: Icons.ShieldCheck };
        }
      case 'oidc':
        return { text: 'OIDC', icon: Icons.ShieldCheck };
      default:
        return { text: user.authType || 'Local', icon: Icons.Key };
    }
  };

  if (!isVisible || !user) return null;

  return (
    <SlidingSidePanel
      isVisible={isVisible}
      slideFrom="right"
      panelWidth={430}
      zIndex={1000}
      skipAnimation={false}
    >
      <Box>
        <Flex justifyContent="space-between" alignItems="flex-start" p={4}>
          <Flex alignItems="center" gap={3}>
            <Icons.UserIdBadge size="extra-large" />
            <Box>
              <Text fontSize={4} fontWeight="bold" lineHeight="1.2" mb={1}>
                {user.name}
              </Text>
              <Flex alignItems="center" gap={2}>
                <Icon as={renderAuthType(user).icon} size="small" />
                <Text fontSize={2} color="text.muted">
                  {renderAuthType(user).text}
                </Text>
              </Flex>
            </Box>
          </Flex>
          <ButtonIcon onClick={onClose} data-testid="info-guide-btn-close">
            <Icons.Cross size="small" />
          </ButtonIcon>
        </Flex>

        <Box>
          {isLoading && <Text>Loading user details...</Text>}

          {userDetails && (
            <>
              <Section>
                <SectionTitle>User details</SectionTitle>
                <Flex flexWrap="wrap" gap={2}>
                  <Box width="48%">
                    <Text fontWeight="medium">Username</Text>
                    <Text color="text.muted">{userDetails.name}</Text>
                  </Box>
                  <Box width="48%">
                    <Text fontWeight="medium">Auth Type</Text>
                    <Text color="text.muted">
                      {renderAuthType(userDetails).text}
                    </Text>
                  </Box>
                  {userDetails.origin === 'okta' && (
                    <Box width="48%">
                      <Text fontWeight="medium">Okta User ID</Text>
                      <Text color="text.muted">01udj4fs0kdXFUYw591</Text>
                    </Box>
                  )}
                  <Box width="48%">
                    <Text fontWeight="medium">Status</Text>
                    <Flex alignItems="center" gap={2}>
                      <Box
                        width="8px"
                        height="8px"
                        borderRadius="50%"
                        backgroundColor="success.main"
                      />
                      <Text color="text.muted">Active</Text>
                    </Flex>
                  </Box>
                </Flex>
              </Section>

              <Section>
                <SectionTitle>Access Lists (0)</SectionTitle>
              </Section>

              <Section>
                <SectionTitle>
                  Roles ({userDetails.roles?.length || 0})
                </SectionTitle>
                {userDetails.roles && userDetails.roles.length > 0 && (
                  <Flex flexWrap="wrap" gap={2}>
                    {userDetails.roles.slice(0, 7).map(role => (
                      <Label key={role} kind="secondary">
                        <Flex alignItems="center" gap={1}>
                          <Icons.UserIdBadge size={12} />
                          {role}
                        </Flex>
                      </Label>
                    ))}
                    {userDetails.roles.length > 7 && (
                      <ClickableLabel kind="secondary" onClick={() => setIsRolesModalOpen(true)}>
                        + {userDetails.roles.length - 7} more
                      </ClickableLabel>
                    )}
                  </Flex>
                )}
              </Section>

              {userDetails.allTraits && (
                <Section>
                  <SectionTitle>Traits</SectionTitle>
                  <Flex flexWrap="wrap" gap={2}>
                    {Object.entries(userDetails.allTraits)
                      .filter(([key, values]) => {
                        if (Array.isArray(values)) {
                          return (
                            values.length > 0 &&
                            values.some(v => v && v.trim() !== '')
                          );
                        }
                        return values && values.trim() !== '';
                      })
                      .map(([key, values]) => (
                        <Label key={key} kind="secondary">
                          {key}:{' '}
                          {Array.isArray(values) ? values.join(', ') : values}
                        </Label>
                      ))}
                  </Flex>
                </Section>
              )}
            </>
          )}
        </Box>
        
        {userDetails && (
          <UserRoles
            isOpen={isRolesModalOpen}
            onClose={() => setIsRolesModalOpen(false)}
            roles={userDetails.roles || []}
            userName={userDetails.name}
          />
        )}
      </Box>
    </SlidingSidePanel>
  );
}

const Section = styled.section`
  border-bottom: 1px solid ${props => props.theme.colors.levels.sunken};
  padding: 0 ${props => props.theme.space[4]}px
    ${props => props.theme.space[4]}px ${props => props.theme.space[4]}px;
  margin-bottom: ${props => props.theme.space[4]}px;

  &:last-child {
    border-bottom: none;
    margin-bottom: 0;
  }
`;

const SectionTitle = styled(Text).attrs({
  fontSize: 3,
  fontWeight: 'bold',
  mb: 2,
})``;

const ClickableLabel = styled(Label)`
  cursor: pointer;
  
  &:hover {
    background: ${props => props.theme.colors.levels.elevated};
  }
`;
