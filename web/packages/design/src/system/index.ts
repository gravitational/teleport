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

import { Property } from 'csstype';
import {
  alignItems,
  alignSelf,
  background,
  border,
  borderColor,
  borderRadius,
  borders,
  color,
  flex,
  flexBasis,
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
  ResponsiveValue,
  size,
  space,
  style,
  textAlign,
  TLengthStyledSystem,
  width,
  type AlignItemsProps,
  type AlignSelfProps,
  type BackgroundProps,
  type BorderColorProps,
  type BorderProps,
  type BorderRadiusProps,
  type BordersProps,
  type ColorProps,
  type FlexBasisProps,
  type FlexDirectionProps,
  type FlexProps,
  type FlexWrapProps,
  type FontSizeProps,
  type FontWeightProps,
  type HeightProps,
  type JustifyContentProps,
  type JustifySelfProps,
  type LineHeightProps,
  type MaxHeightProps,
  type MaxWidthProps,
  type MinHeightProps,
  type MinWidthProps,
  type OverflowProps,
  type SizeProps,
  type SpaceProps,
  type TextAlignProps,
  type WidthProps,
} from 'styled-system';

import typography, { type TypographyProps } from './typography';

export const gap = style({
  prop: 'gap',
  cssProperty: 'gap',
  // This makes gap use the space defined in the theme.
  // https://github.com/styled-system/styled-system/blob/v3.1.11/src/index.js#L67
  key: 'space',
});

export interface GapProps<TLength = TLengthStyledSystem> {
  gap?: ResponsiveValue<Property.Gap<TLength>>;
}

export const rowGap = style({
  prop: 'rowGap',
  cssProperty: 'row-gap',
  key: 'space',
});

export interface RowGapProps<TLength = TLengthStyledSystem> {
  rowGap?: ResponsiveValue<Property.RowGap<TLength>>;
}

export const columnGap = style({
  prop: 'columnGap',
  cssProperty: 'column-gap',
  key: 'space',
});

export interface ColumnGapProps<TLength = TLengthStyledSystem> {
  columnGap?: ResponsiveValue<Property.ColumnGap<TLength>>;
}

// Defining a custom style for boxShadow, as the one from styled-system doesn't support reading
// values from the theme.
export const boxShadow = style({
  prop: 'boxShadow',
  cssProperty: 'box-shadow',
  key: 'boxShadow',
});

export interface BoxShadowProps<TLength = TLengthStyledSystem> {
  // Using Property.Gap on purpose, as Property.BoxShadow doesn't accept a generic value (so it
  // seemingly doesn't accept passing an index for a value from the theme).
  boxShadow?: ResponsiveValue<Property.Gap<TLength>>;
}

export {
  alignItems,
  type AlignItemsProps,
  alignSelf,
  type AlignSelfProps,
  background,
  type BackgroundProps,
  border,
  type BorderProps,
  borderColor,
  type BorderColorProps,
  borders,
  type BordersProps,
  borderRadius,
  type BorderRadiusProps,
  color,
  type ColorProps,
  flex,
  type FlexProps,
  flexBasis,
  type FlexBasisProps,
  flexDirection,
  type FlexDirectionProps,
  flexWrap,
  type FlexWrapProps,
  fontSize,
  type FontSizeProps,
  fontWeight,
  type FontWeightProps,
  height,
  type HeightProps,
  justifyContent,
  type JustifyContentProps,
  justifySelf,
  type JustifySelfProps,
  lineHeight,
  type LineHeightProps,
  maxHeight,
  type MaxHeightProps,
  maxWidth,
  type MaxWidthProps,
  minHeight,
  type MinHeightProps,
  minWidth,
  type MinWidthProps,
  overflow,
  type OverflowProps,
  size,
  type SizeProps,
  space,
  type SpaceProps,
  textAlign,
  type TextAlignProps,
  typography,
  type TypographyProps,
  width,
  type WidthProps,
};
