/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { getContrastRatio } from '../utils/colorManipulator';
import { lightBlue, blueGrey, yellow } from '../palette';
import typography, { fontSizes, fontWeights } from '../typography';
import { fonts } from '../fonts';

import { SharedColors, SharedStyles } from './types';

// TODO(bl-nero): use a CSS var for sidebar width and make the breakpoints work
// by changing the minimum width on a per-view basis (Main.tsx).
const sidebarWidth = 256;

// Styles that are shared by all themes.
export const sharedStyles: SharedStyles = {
  boxShadow: [
    '0px 2px 1px -1px rgba(0, 0, 0, 0.2), 0px 1px 1px rgba(0, 0, 0, 0.14), 0px 1px 3px rgba(0, 0, 0, 0.12)',
    '0px 5px 5px -3px rgba(0, 0, 0, 0.2), 0px 8px 10px 1px rgba(0, 0, 0, 0.14), 0px 3px 14px 2px rgba(0, 0, 0, 0.12)',
    '0px 3px 5px -1px rgba(0, 0, 0, 0.2), 0px 6px 10px rgba(0, 0, 0, 0.14), 0px 1px 18px rgba(0, 0, 0, 0.12)',
  ],
  breakpoints: {
    mobile: 400 + sidebarWidth,
    tablet: 800 + sidebarWidth,
    desktop: 1200 + sidebarWidth,
  },
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
  radii: [0, 2, 4, 8, 16, 9999, '100%'],
  regular: fontWeights.regular,
  bold: fontWeights.bold,
};

// Colors that are shared between all themes, these should be added to the theme.colors object.
export const sharedColors: SharedColors = {
  dark: '#000000',
  light: '#FFFFFF',
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
