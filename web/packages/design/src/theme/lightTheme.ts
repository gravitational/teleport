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

import { fonts } from './fonts';
import { darken, getContrastRatio } from './utils/colorManipulator';
import { lightBlue, blueGrey, yellow } from './palette';
import typography, { fontSizes, fontWeights } from './typography';
import { sharedStyles } from './sharedStyles';

const space = [0, 4, 8, 16, 24, 32, 40, 48, 56, 64, 72, 80];
const contrastThreshold = 3;

const colors = {
  /*
  Colors in `levels` are used to reflect the perceived depth of elements in the UI.
  The further back an element is, the more "sunken" it is, and the more forwards it is, the more "elevated" it is (think CSS z-index).

  A `sunken` color would be used to represent something like the background of the app.
  While `surface` would be the color of the primary surface where most content is located (such as tables).
  Any colors more "elevated" than that would be used for things such as popovers, menus, and dialogs.

  For more information on this concept: https://m3.material.io/styles/elevation/applying-elevation
 */
  levels: {
    deep: '#E6E9EA',

    sunken: '#F1F2F4',

    surface: '#FBFBFC',

    elevated: '#FFFFFF',

    popout: '#FFFFFF',
  },

  // Spot backgrounds are used as highlights, for example
  // to indicate a hover or active state for an item in a menu.
  spotBackground: ['rgba(0,0,0,0.06)', 'rgba(0,0,0,0.13)', 'rgba(0,0,0,0.18)'],

  brand: '#512FC9',

  text: {
    // The most important text.
    main: '#000000',
    // Slightly muted text.
    slightlyMuted: 'rgba(0,0,0,0.72)',
    // Muted text. Also used as placeholder text in forms.
    muted: 'rgba(0,0,0,0.54)',
    // Disabled text.
    disabled: 'rgba(0,0,0,0.36)',
    // For text on  a background that is on a color opposite to the theme. For dark theme,
    // this would mean text that is on a light background.
    primaryInverse: '#FFFFFF',
  },

  buttons: {
    text: '#000000',
    textDisabled: 'rgba(0,0,0,0.3)',
    bgDisabled: 'rgba(0,0,0,0.12)',

    primary: {
      text: '#FFFFFF',
      default: '#512FC9',
      hover: '#4126A1',
      active: '#311C79',
    },

    secondary: {
      default: 'rgba(0,0,0,0.07)',
      hover: 'rgba(0,0,0,0.13)',
      active: 'rgba(0,0,0,0.18)',
    },

    border: {
      default: 'rgba(255,255,255,0)',
      hover: 'rgba(0,0,0,0.07)',
      active: 'rgba(0,0,0,0.13)',
      border: 'rgba(0,0,0,0.36)',
    },

    warning: {
      text: '#FFFFFF',
      default: '#CC372D',
      hover: '#A32C24',
      active: '#7A211B',
    },

    trashButton: {
      default: 'rgba(0,0,0,0.07)',
      hover: 'rgba(0,0,0,0.13)',
    },

    link: {
      default: '#0073BA',
      hover: '#005C95',
      active: '#004570',
    },
  },

  tooltip: {
    background: '#F0F2F4',
  },

  progressBarColor: '#007D6B',

  dark: '#000000',
  light: '#FFFFFF',

  grey: {
    ...blueGrey,
  },

  error: {
    main: '#CC372D',
    hover: '#A32C24',
    active: '#7A211B',
  },

  warning: {
    main: '#FFAB00',
    hover: '#CC8900',
    active: '#996700',
  },

  action: {
    active: '#FFFFFF',
    hover: 'rgba(255, 255, 255, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(255, 255, 255, 0.2)',
    disabled: 'rgba(255, 255, 255, 0.3)',
    disabledBackground: 'rgba(255, 255, 255, 0.12)',
  },

  terminal: {
    foreground: '#000',
    background: '#F1F2F4', // levels.sunken
    selectionBackground: 'rgba(0, 0, 0, 0.18)',
    cursor: '#000',
    cursorAccent: '#F1F2F4',
    red: '#9D0A00',
    green: '#005742',
    yellow: '#704B00',
    blue: '#004B89',
    magenta: '#3D1BB2',
    cyan: '#015C6E',
    brightWhite: darken('#F1F2F4', 0.55),
    white: darken('#F1F2F4', 0.68),
    brightBlack: darken('#F1F2F4', 0.8),
    black: '#000',
    brightRed: '#BF372E',
    brightGreen: '#007562',
    brightYellow: '#8F5F00',
    brightBlue: '#006BB8',
    brightMagenta: '#5531D4',
    brightCyan: '#007282',
  },

  subtle: blueGrey[50],
  link: '#0073BA',
  bgTerminal: '#010B1C',
  highlight: yellow[50],
  disabled: blueGrey[500],
  info: lightBlue[600],
  success: '#007D6B',
};

const borders = [
  0,
  '1px solid',
  '2px solid',
  '4px solid',
  '8px solid',
  '16px solid',
  '32px solid',
];

const theme = {
  name: 'light',
  colors,
  typography,
  font: fonts.sansSerif,
  fonts: fonts,
  fontWeights,
  fontSizes,
  space,
  borders,
  radii: [0, 2, 4, 8, 16, 9999, '100%'],
  regular: fontWeights.regular,
  bold: fontWeights.bold,
  ...sharedStyles,
  // disabled media queries for styled-system
  breakpoints: [],
};

export default theme;

export function getContrastText(background) {
  // Use the same logic as
  // Bootstrap: https://github.com/twbs/bootstrap/blob/1d6e3710dd447de1a200f29e8fa521f8a0908f70/scss/_functions.scss#L59
  // and material-components-web https://github.com/material-components/material-components-web/blob/ac46b8863c4dab9fc22c4c662dc6bd1b65dd652f/packages/mdc-theme/_functions.scss#L54
  const contrastText =
    getContrastRatio(background, colors.light) >= contrastThreshold
      ? colors.light
      : colors.dark;

  return contrastText;
}
