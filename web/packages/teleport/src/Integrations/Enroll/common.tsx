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

import styled from 'styled-components';

import { Box, Flex, H2, ResourceIcon } from 'design';
import { P } from 'design/Text/Text';

export const IntegrationTile = styled(Flex)<{
  disabled?: boolean;
  $exists?: boolean;
}>`
  color: inherit;
  text-decoration: none;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  border-radius: ${({ theme }) => theme.radii[2]}px;
  padding: ${({ theme }) => theme.space[3]}px;
  gap: ${({ theme }) => theme.space[3]}px;
  height: 170px;
  width: 170px;
  background-color: ${({ theme }) => theme.colors.buttons.secondary.default};
  text-align: center;
  cursor: ${({ disabled, $exists }) =>
    disabled || $exists ? 'not-allowed' : 'pointer'};
  transition: background-color 200ms ease;

  ${props => {
    if (props.$exists) {
      return;
    }

    return `
    opacity: ${props.disabled ? '0.45' : '1'};
    &:hover,
    &:focus-visible {
      background-color: ${props.theme.colors.buttons.secondary.hover};
    }
    `;
  }};
`;

export const NoCodeIntegrationDescription = () => (
  <Box mb={3}>
    <H2 mb={1}>No-Code Integrations</H2>
    <P>
      Set up Teleport to post notifications to messaging apps, discover and
      import resources from cloud providers and other services.
    </P>
  </Box>
);

/**
 * IntegrationIcon wraps ResourceIcon with css required for integration
 * and plugin tiles.
 */
export const IntegrationIcon = styled(ResourceIcon)<{ size?: number }>`
  display: inline-block;
  margin: 0 auto;
  height: 100%;
  min-width: 0;
  ${({ size }) => size && `max-width: ${size}px;`}
`;
