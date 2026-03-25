/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ComponentPropsWithoutRef } from 'react';
import styled, { DefaultTheme, useTheme } from 'styled-components';

import Box from 'design/Box';
import Flex, { Stack } from 'design/Flex';
import { darken, emphasize } from 'design/theme/utils/colorManipulator';

/**
 * DemoTerminal is meant for showing examples of terminal output.
 */
export const DemoTerminal = (
  props: { text: string; title: string } & ComponentPropsWithoutRef<
    typeof Stack
  >
) => {
  const { text, title, ...stackProps } = props;
  const theme = useTheme();
  return (
    <Terminal {...stackProps}>
      <TopBar>
        <Flex gap={2}>
          <CircleButton
            $color={theme.colors.interactive.solid.danger.default}
          />
          <CircleButton $color={theme.colors.interactive.solid.alert.default} />
          <CircleButton
            $color={theme.colors.interactive.solid.success.default}
          />
        </Flex>
        <Title>{title}</Title>
      </TopBar>
      <Box width="100%" flex={1} p={2} py={1}>
        <Pre>{text}</Pre>
      </Box>
    </Terminal>
  );
};

const Terminal = styled(Stack).attrs({ borderRadius: 2, gap: 0 })`
  background-color: ${props => props.theme.colors.levels.deep};
  border: 1px solid ${props => topBarColor(props.theme)};
  font-family: ${props => props.theme.fonts.mono};
`;

const TopBar = styled(Box).attrs({ py: 1, px: 2, width: '100%' })`
  background-color: ${props => topBarColor(props.theme)};
  display: grid;
  // minmax(0, min-content) is important to let the title cell shrink if necessary.
  // Using 1fr on both sides of the title cell centers the title cell while still allowing a gap to
  // form between the buttons and the title on narrow widths.
  grid-template-columns: 1fr minmax(0, min-content) 1fr;
  grid-template-rows: 1fr;
  gap: ${props => props.theme.space[2]}px;
  align-items: center;
`;

const Title = styled.span`
  text-align: center;
  justify-self: center;

  // Make sure there's only a single line with an ellipsis at the end if the title cannot fit on a
  // single line.
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const Pre = styled.pre`
  margin: 0;
  white-space: pre-wrap;
  // Simulate how terminal emulators hard break lines.
  word-break: break-all;
`;

const CircleButton = styled(Box)<{ $color: string }>`
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background-color: ${props => props.$color};
  border: 1px solid ${props => darken(props.$color, 0.2)};
`;

const topBarColor = (theme: DefaultTheme): string =>
  emphasize(theme.colors.levels.deep, 0.2);
