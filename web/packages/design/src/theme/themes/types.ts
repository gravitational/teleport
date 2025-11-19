/**
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

import type { LegacyThemeColors } from '@gravitational/design-system';

import { Fonts } from '../fonts';
import { blueGrey } from '../palette';
import typography, { fontSizes, fontWeights } from '../typography';

export type SharedColors = {
  dark: string;
  light: string;
  interactionHandle: string;
  grey: typeof blueGrey;
  subtle: string;
  bgTerminal: string;
  highlight: string;
  disabled: string;
  info: string;
};

export type SharedStyles = {
  sidebarWidth: number;
  boxShadow: string[];
  breakpoints: {
    /** @deprecated Use the "small" breakpoint instead. */
    mobile: string;
    /** @deprecated Use the "medium" breakpoint instead. */
    tablet: string;
    /** @deprecated Use the "large" breakpoint instead. */
    desktop: string;
    small: string;
    medium: string;
    large: string;
    700: string;
    900: string;
    1200: string;
  };
  topBarHeight: number[];
  /**
   *
   * idx:    0  1  2   3   4   5   6   7   8   9  10  11
   * space: [0, 4, 8, 16, 24, 32, 40, 48, 56, 64, 72, 80]
   */
  space: number[];
  borders: (string | number)[];
  typography: typeof typography;
  font: string;
  fonts: Fonts;
  fontWeights: typeof fontWeights;
  fontSizes: typeof fontSizes;
  radii: (number | string)[];
  regular: number;
  bold: number;
};

export type Theme = {
  name: string;
  /** This field should be either `light` or `dark`. This is used to determine things like which version of logos to use
  so that they contrast properly with the background. */
  type: 'dark' | 'light';
  /** Whether this is a custom theme and not Dark Theme/Light Theme. */
  isCustomTheme: boolean;
  colors: LegacyThemeColors & SharedColors;
} & SharedStyles;

export type ThemeDefinition = Omit<Theme, 'colors'>;
