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
  AlignSelfProps,
  BorderColorProps,
  BorderProps,
  BordersProps,
  ColorProps,
  FlexProps,
  HeightProps,
  JustifySelfProps,
  LineHeightProps,
  MaxHeightProps,
  MaxWidthProps,
  MinHeightProps,
  MinWidthProps,
  OverflowProps,
  SpaceProps,
  TextAlignProps,
  WidthProps,
} from 'styled-system';

import {
  overflow,
  borders,
  borderRadius,
  BorderRadiusProps,
  borderColor,
  flex,
  height,
  lineHeight,
  maxWidth,
  minHeight,
  maxHeight,
  minWidth,
  alignSelf,
  justifySelf,
  space,
  width,
  color,
  textAlign,
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
    BorderRadiusProps,
    OverflowProps,
    BorderColorProps {}

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
  ${borderRadius}
  ${overflow}
  ${borderColor}
`;

Box.displayName = 'Box';

export default Box;
