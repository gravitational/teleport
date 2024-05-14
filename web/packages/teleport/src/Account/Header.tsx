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

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Text,
} from 'design';
import React from 'react';
import styled, { useTheme, css } from 'styled-components';

export interface HeaderProps {
  title: React.ReactNode;
  description?: string;
  icon: React.ReactNode;
  showIndicator?: boolean;
  actions: React.ReactNode;
}

export function Header({
  title,
  description,
  icon,
  showIndicator,
  actions,
}: HeaderProps) {
  const theme = useTheme();
  return (
    <Flex alignItems="center" gap={3}>
      {/* lineHeight=0 prevents the icon background from being larger than
          required by the icon itself. */}
      <Box
        bg={theme.colors.interactive.tonal.neutral[0]}
        lineHeight={0}
        p={2}
        borderRadius={3}
      >
        {icon}
      </Box>
      <Box flex="1">
        <Text typography="h4">{title}</Text>
        <Text typography="body1" color={theme.colors.text.slightlyMuted}>
          {description}
        </Text>
      </Box>
      {/* Indicator is always in the layout so that the description text doesn't
          reflow if visibility changes. */}
      <Box
        lineHeight={0}
        style={{ visibility: showIndicator ? 'visible' : 'hidden' }}
      >
        <Indicator size={40} />
      </Box>
      <Box flex="0 0 auto">{actions}</Box>
    </Flex>
  );
}

const actionButtonStyles = css`
  padding: ${props => `${props.theme.space[2]}px ${props.theme.space[4]}px`};
  gap: ${props => `${props.theme.space[2]}px`};
  text-transform: none;
`;

export const ActionButtonSecondary = styled(ButtonSecondary)`
  ${actionButtonStyles}
`;

export const ActionButtonPrimary = styled(ButtonPrimary)`
  ${actionButtonStyles}
`;
