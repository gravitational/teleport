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

import React from 'react';
import styled, { css, useTheme } from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Indicator,
  Subtitle2,
} from 'design';

export interface HeaderProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  icon: React.ReactNode;
  showIndicator?: boolean;
  actions: React.ReactNode;
  /**
   * Position of the action element in the Header component.
   * - 'top' - action is aligned to the top of the header
   * - 'center' - action is vertically aligned to the center of the Header (default)
   */
  actionPosition?: 'top' | 'center';
}

export function Header({
  title,
  description,
  icon,
  showIndicator,
  actions,
  actionPosition = 'center',
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
        alignSelf={'flex-start'}
      >
        {icon}
      </Box>
      <ContentFlex>
        <Box flex="1">
          <H2>{title}</H2>
          <Subtitle2 color={theme.colors.text.slightlyMuted}>
            {description}
          </Subtitle2>
        </Box>
        <ActionFlex actionPosition={actionPosition}>
          {/* Indicator is always in the layout so that the description text doesn't
              reflow if visibility changes. */}
          <Box
            lineHeight={0}
            style={{ visibility: showIndicator ? 'visible' : 'hidden' }}
            data-testid="indicator-wrapper"
            height={40}
            width={40}
          >
            <Indicator size={40} delay="none" />
          </Box>
          <ActionBox>{actions}</ActionBox>
        </ActionFlex>
      </ContentFlex>
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

const ActionBox = styled(Box)`
  flex: 0 0 auto;

  @media (max-width: ${props => props.theme.breakpoints.medium}) {
    flex: 1 1 100% !important;
  }
`;

const ContentFlex = styled(Flex)`
  flex-direction: row;
  flex-wrap: wrap;
  gap: ${props => props.theme.space[3]}px;
  flex: 1 1 auto;

  @media (max-width: ${props => props.theme.breakpoints.medium}) {
    & > * {
      flex: 1 1 100% !important;
    }
  }
`;

const ActionFlex = styled(Flex)<{
  actionPosition?: HeaderProps['actionPosition'];
}>`
  flex-direction: row;
  align-self: ${props =>
    props.actionPosition === 'top' ? 'flex-start' : 'center'};
  align-items: ${props =>
    props.actionPosition === 'top' ? 'flex-start' : 'center'};
  gap: ${props => props.theme.space[2]}px;
  flex: 0 0 auto;

  @media (max-width: ${props => props.theme.breakpoints.medium}) {
    flex-direction: row-reverse;
  }
`;
