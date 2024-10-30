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
import { Box, Flex, Text } from 'design';
import styled from 'styled-components';

export const IntegrationTile = styled(Flex)<{
  disabled?: boolean;
  $exists?: boolean;
}>`
  color: inherit;
  text-decoration: none;
  flex-direction: column;
  align-items: center;
  position: relative;
  border-radius: 4px;
  height: 170px;
  width: 170px;
  background-color: ${({ theme }) => theme.colors.buttons.secondary.default};
  text-align: center;
  cursor: pointer;

  ${props => {
    const pointerEvents = props.disabled || props.$exists ? 'none' : 'auto';
    if (props.$exists) {
      return { pointerEvents };
    }

    return `
    opacity: ${props.disabled ? '0.45' : '1'};
    &:hover {
      background-color: ${props.theme.colors.buttons.secondary.hover};
    }
    pointer-events: ${pointerEvents};
    `;
  }}
`;

export const NoCodeIntegrationDescription = () => (
  <Box mb={3}>
    <Text fontWeight="bold" typography="h4">
      No-Code Integrations
    </Text>
    <Text typography="body1">
      Set up Teleport to post notifications to messaging apps, discover and
      import resources from cloud providers and other services.
    </Text>
  </Box>
);
