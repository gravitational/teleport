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

import {
  alignItems,
  alignSelf,
  border,
  borderColor,
  borders,
  color,
  flex,
  flexDirection,
  flexWrap,
  fontSize,
  fontWeight,
  height,
  justifyContent,
  justifySelf,
  lineHeight,
  maxHeight,
  maxWidth,
  minHeight,
  minWidth,
  overflow,
  size,
  space,
  textAlign,
  width,
  style,
  ResponsiveValue,
  TLengthStyledSystem,
} from 'styled-system';

import { Property } from 'csstype';

import typography from './typography';
import borderRadius, { BorderRadiusProps } from './borderRadius';

const gap = style({
  prop: 'gap',
  cssProperty: 'gap',
  // This makes gap use the space defined in the theme.
  // https://github.com/styled-system/styled-system/blob/v3.1.11/src/index.js#L67
  key: 'space',
});

export interface GapProps<TLength = TLengthStyledSystem> {
  gap?: ResponsiveValue<Property.Gap<TLength>>;
}

export {
  alignItems,
  alignSelf,
  border,
  borderColor,
  borders,
  borderRadius,
  type BorderRadiusProps,
  color,
  flex,
  flexDirection,
  flexWrap,
  fontSize,
  fontWeight,
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
  size,
  space,
  textAlign,
  typography,
  width,
};
