/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import React, { type RefAttributes } from 'react';
import styled from 'styled-components';

import {
  alignSelf,
  color,
  height,
  maxHeight,
  maxWidth,
  space,
  width,
} from 'design/system';
import type {
  AlignSelfProps,
  ColorProps,
  HeightProps,
  MaxHeightProps,
  MaxWidthProps,
  SpaceProps,
  WidthProps,
} from 'design/system';

export interface ThemedImageProps
  extends
    SpaceProps,
    ColorProps,
    WidthProps,
    HeightProps,
    MaxWidthProps,
    MaxHeightProps,
    AlignSelfProps,
    RefAttributes<HTMLImageElement> {
  /** Asset URL used under the dark theme. */
  dark: string;
  /** Asset URL used under the light theme. */
  light: string;
  alt?: string;
  style?: React.CSSProperties;
  className?: string;
  'data-testid'?: string;
}

/**
 * Renders a themed resource icon whose dark/light artworks genuinely differ.
 *
 * It behaves exactly like a plain `<img>` (so intrinsic sizing and single-axis sizing
 * work as before), but swaps the rendered asset purely via the theme class on `<html>`
 * using CSS `content`, so it never subscribes to the theme context.
 * The dark asset is the default `src`; under the light theme, CSS swaps in the light
 * asset.
 */
export function ThemedImage({ dark, light, alt, ...rest }: ThemedImageProps) {
  return <StyledImg src={dark} alt={alt} $light={light} {...rest} />;
}

type StyledImgProps = SpaceProps &
  ColorProps &
  WidthProps &
  HeightProps &
  MaxWidthProps &
  MaxHeightProps &
  AlignSelfProps & { $light: string };

const StyledImg = styled.img<StyledImgProps>`
  display: block;
  outline: none;
  ${color} ${space} ${width} ${height} ${maxWidth} ${maxHeight} ${alignSelf}

  html.light & {
    content: url("${p => p.$light}");
  }
`;
