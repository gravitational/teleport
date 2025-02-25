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

import {
  alignSelf,
  AlignSelfProps,
  BorderProps,
  borders,
  BordersProps,
  color,
  ColorProps,
  flex,
  FlexProps,
  height,
  HeightProps,
  justifySelf,
  JustifySelfProps,
  lineHeight,
  LineHeightProps,
  maxHeight,
  MaxHeightProps,
  maxWidth,
  MaxWidthProps,
  minHeight,
  MinHeightProps,
  minWidth,
  MinWidthProps,
  overflow,
  OverflowProps,
  space,
  SpaceProps,
  textAlign,
  TextAlignProps,
  width,
  WidthProps,
} from '../system';

export interface BoxProps
  extends MaxWidthProps,
    MinWidthProps,
    SpaceProps,
    HeightProps,
    LineHeightProps,
    MinHeightProps,
    MaxHeightProps,
    WidthProps,
    ColorProps,
    TextAlignProps,
    FlexProps,
    AlignSelfProps,
    JustifySelfProps,
    BorderProps,
    BordersProps,
    OverflowProps {}

const Box = styled.div<BoxProps>`
  box-sizing: border-box;
  ${maxWidth}
  ${minWidth}
  ${space}
  ${height}
  ${lineHeight}
  ${minHeight}
  ${maxHeight}
  ${width}
  ${color}
  ${textAlign}
  ${flex}
  ${alignSelf}
  ${justifySelf}
  ${borders}
  ${overflow}
`;

Box.displayName = 'Box';

export default Box;
