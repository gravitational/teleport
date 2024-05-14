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

import { getContrastRatio } from '../utils/colorManipulator';
import { lightBlue, blueGrey, yellow } from '../palette';
import typography, { fontSizes, fontWeights } from '../typography';
import { fonts } from '../fonts';

import { SharedColors, SharedStyles } from './types';

const dockedAssistWidth = 520;
// TODO(bl-nero): use a CSS var for sidebar width and make the breakpoints work
// by changing the minimum width on a per-view basis (Main.tsx).
const sidebarWidth = 256;

// Styles that are shared by all themes.
export const sharedStyles: SharedStyles = {
  dockedAssistWidth,
  sidebarWidth,
  boxShadow: [
    '0px 2px 1px -1px rgba(0, 0, 0, 0.2), 0px 1px 1px rgba(0, 0, 0, 0.14), 0px 1px 3px rgba(0, 0, 0, 0.12)',
    '0px 5px 5px -3px rgba(0, 0, 0, 0.2), 0px 8px 10px 1px rgba(0, 0, 0, 0.14), 0px 3px 14px 2px rgba(0, 0, 0, 0.12)',
    '0px 3px 5px -1px rgba(0, 0, 0, 0.2), 0px 6px 10px rgba(0, 0, 0, 0.14), 0px 1px 18px rgba(0, 0, 0, 0.12)',
    '0px 1px 10px 0px rgba(0, 0, 0, 0.12), 0px 4px 5px 0px rgba(0, 0, 0, 0.14), 0px 2px 4px -1px rgba(0, 0, 0, 0.20)',
  ],
  breakpoints: {
    // TODO (avatus): remove mobile/tablet/desktop breakpoints in favor of screensize descriptions
    mobile: 400 + sidebarWidth,
    tablet: 800 + sidebarWidth,
    desktop: 1200 + sidebarWidth,
    // use these from now on
    small: 600,
    medium: 1024,
    large: 1280,
  },
  topBarHeight: [44, 56, 72],
  space: [0, 4, 8, 16, 24, 32, 40, 48, 56, 64, 72, 80],
  borders: [
    0,
    '1px solid',
    '2px solid',
    '4px solid',
    '8px solid',
    '16px solid',
    '32px solid',
  ],
  typography,
  font: fonts.sansSerif,
  fonts: fonts,
  fontWeights,
  fontSizes,
  // TODO(rudream): Clean up radii order in sharedStyles.
  radii: [0, 2, 4, 8, 16, 9999, '100%', 24],
  regular: fontWeights.regular,
  bold: fontWeights.bold,
};

// Colors that are shared between all themes, these should be added to the theme.colors object.
export const sharedColors: SharedColors = {
  dark: '#000000',
  light: '#FFFFFF',
  interactionHandle: '#FFFFFF',
  grey: {
    ...blueGrey,
  },
  subtle: blueGrey[50],
  bgTerminal: '#010B1C',
  highlight: yellow[50],
  disabled: blueGrey[500],
  info: lightBlue[600],
};

export function getContrastText(background) {
  // Use the same logic as
  // Bootstrap: https://github.com/twbs/bootstrap/blob/1d6e3710dd447de1a200f29e8fa521f8a0908f70/scss/_functions.scss#L59
  // and material-components-web https://github.com/material-components/material-components-web/blob/ac46b8863c4dab9fc22c4c662dc6bd1b65dd652f/packages/mdc-theme/_functions.scss#L54
  const contrastText =
    getContrastRatio(background, '#FFFFFF') >= 3 ? '#FFFFFF' : '#000000';

  return contrastText;
}
