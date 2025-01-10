/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { pluralize } from 'shared/utils/text';

import { AssumedRequest } from 'teleterm/services/tshd/types';

import { useAssumedRolesBar } from './useAssumedRolesBar';

export function AssumedRolesBar({ assumedRolesRequest }: Props) {
  const {
    duration,
    assumedRoles,
    dropRequest,
    dropRequestAttempt,
    hasExpired,
  } = useAssumedRolesBar(assumedRolesRequest);
  const roleText = pluralize(assumedRoles.length, 'role');
  const durationText = `${roleText} assumed, expires in ${duration}`;
  const hasExpiredText =
    assumedRoles.length > 1 ? 'have expired' : 'has expired';
  const expirationText = `${roleText} ${hasExpiredText}`;
  const assumedRolesText = assumedRoles.join(', ');
  return (
    <Box
      px={3}
      py={2}
      bg="brand"
      borderTop={1}
      css={`
        border-color: ${props => props.theme.colors.spotBackground[1]};
      `}
    >
      <Flex justifyContent="space-between" alignItems="center">
        <Flex alignItems="center">
          <Box
            borderRadius="20px"
            py={1}
            px={3}
            mr={2}
            color="text.primary"
            bg="text.primaryInverse"
            style={{
              fontWeight: '500',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              maxWidth: '200px',
              whiteSpace: 'nowrap',
            }}
            title={assumedRolesText}
          >
            {assumedRolesText}
          </Box>
          <Text color="text.primaryInverse">
            {hasExpired ? expirationText : durationText}
          </Text>
        </Flex>
        <StyledButtonLink
          onClick={dropRequest}
          disabled={dropRequestAttempt.status === 'processing'}
        >
          Drop Request
        </StyledButtonLink>
      </Flex>
    </Box>
  );
}

type Props = {
  assumedRolesRequest: AssumedRequest;
};

const StyledButtonLink = styled.button`
  color: ${props => props.theme.colors.text.primaryInverse};
  background: none;
  text-decoration: underline;
  text-transform: none;
  padding: 8px;
  outline: none;
  border: none;
  border-radius: 4px;
  font-family: inherit;

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.spotBackground[1]};
    cursor: pointer;
  }

  &:disabled {
    background: ${props => props.theme.colors.spotBackground[0]};
    color: ${props => props.theme.colors.text.disabled};
  }
`;
