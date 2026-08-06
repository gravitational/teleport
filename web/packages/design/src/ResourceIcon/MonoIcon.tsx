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

import React, { type PropsWithChildren, type RefAttributes } from 'react';
import styled from 'styled-components';

import { space, type SpaceProps } from 'design/system';

/**
 * Props shared by every generated monochrome resource icon component.
 *
 * A monochrome resource icon is a single `currentColor` SVG. Its color is driven entirely
 * by CSS via the theme class on the `<html>` element (set by `ConfiguredThemeProvider`),
 * so it never subscribes to the theme context.
 */
export type MonoIconProps = SpaceProps &
  RefAttributes<SVGSVGElement> & {
    /** SVG width. Defaults to the viewBox width. */
    width?: number | string;
    /** SVG height. Defaults to the viewBox height. */
    height?: number | string;
    /**
     * Color used under the light theme. Defaults to black. Generated components pass a
     * brand color here when the icon is not pure black/white.
     */
    lightColor?: string;
    /** Color used under the dark theme. Defaults to white. */
    darkColor?: string;
    viewBox?: string;
    className?: string;
    style?: React.CSSProperties;
    role?: string;
    title?: string;
    'data-testid'?: string;
  };

/**
 * Base component for every generated monochrome resource icon. Renders an
 * inline `currentColor` `<svg>` and applies the per-theme color via the
 * `html.dark` / `html.light` class, so theming is pure CSS.
 */
export function MonoIcon({
  width,
  height,
  lightColor = '#000',
  darkColor = '#fff',
  viewBox = '0 0 24 24',
  children,
  title,
  ...otherProps
}: PropsWithChildren<MonoIconProps>) {
  // Mirror <img> sizing: if only one dimension is given, let the SVG derive the other
  // from its viewBox aspect ratio; if neither is given, fall back to the viewBox's
  // intrinsic size.
  const [, , vbWidth, vbHeight] = viewBox.split(/\s+/);
  const svgWidth = width ?? (height == null ? vbWidth : undefined);
  const svgHeight = height ?? (width == null ? vbHeight : undefined);

  return (
    <Svg
      $light={lightColor}
      $dark={darkColor}
      fill="currentColor"
      width={svgWidth}
      height={svgHeight}
      viewBox={viewBox}
      {...otherProps}
    >
      {title ? <title>{title}</title> : null}
      {children}
    </Svg>
  );
}

const Svg = styled.svg<{ $light: string; $dark: string }>`
  color: ${p => p.$dark};

  html.light & {
    color: ${p => p.$light};
  }

  ${space};
`;
