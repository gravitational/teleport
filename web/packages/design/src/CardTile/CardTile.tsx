/*
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

import { BoxProps } from 'design/Box';
import Flex, { FlexProps } from 'design/Flex';
import {
  alignItems,
  alignSelf,
  borders,
  boxShadow,
  color,
  columnGap,
  flex,
  flexBasis,
  flexDirection,
  flexWrap,
  gap,
  height,
  justifyContent,
  justifySelf,
  lineHeight,
  maxHeight,
  maxWidth,
  minHeight,
  minWidth,
  overflow,
  rowGap,
  space,
  textAlign,
  width,
} from 'design/system';

export const CardTile = styled(Flex)<
  FlexProps & BoxProps & { withBorder?: boolean }
>`
  padding: ${p => p.theme.space[4]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
  flex-basis: 100%;
  min-width: 0;
  background-color: ${p => p.theme.colors.levels.surface};
  border: ${p =>
    p.withBorder
      ? `1px solid ${p.theme.colors.interactive.tonal.neutral[2]}`
      : 'none'};
  box-shadow: ${p => p.theme.boxShadow[0]};

  &:is(a) {
    text-decoration: none;
    color: ${p => p.theme.colors.text.main};
    &:hover {
      background-color: ${p => p.theme.colors.levels.elevated};
      box-shadow: ${p => p.theme.boxShadow[2]};
    }
  }

  ${maxWidth}
  ${alignItems}
  ${alignSelf}
  ${borders}
  ${boxShadow}
  ${color}
  ${columnGap}
  ${flexBasis}
  ${flexDirection}
  ${flexWrap}
  ${flex}
  ${gap}
  ${height}
  ${justifyContent}
  ${justifySelf}
  ${lineHeight}
  ${maxHeight}
  ${minHeight}
  ${minWidth}
  ${overflow}
  ${rowGap}
  ${space}
  ${textAlign}
  ${width}
`;
