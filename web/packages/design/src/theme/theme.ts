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
import { getContrastRatio } from './utils/colorManipulator';
import {
  lightBlue,
  red,
  teal,
  orange,
  pink,
  blueGrey,
  yellow,
} from './palette';
import typography, { fontSizes, fontWeights } from './typography';

const space = [0, 4, 8, 16, 24, 32, 40, 48, 56, 64, 72, 80];
const contrastThreshold = 3;

const colors = {
  /*
  Colors in `levels` are used to reflect the perceived depth of elements in the UI.
  The further back an element is, the more "sunken" it is, and the more forwards it is, the more "elevated" it is (think CSS z-index).

  A `sunken` colour would be used to represent something like the background of the app.
  While `surface` would be the color of the primary surface where most content is located (such as tables).
  Any colors more "elevated" than that would be used for things such as popovers, menus, and dialogs.

  `...Secondary` colours are used to differentiate different colors that represent the same depth.

  For more information on this concept: https://m3.material.io/styles/elevation/applying-elevation
 */
  levels: {
    sunken: '#0C143D',
    sunkenSecondary: '#111B48',

    surface: '#222C59',
    surfaceSecondary: '#1C254D',

    elevated: '#2C3A73',

    popout: '#3E4B7E',
    popoutHighlighted: '#535c8a',
  },

  brand: {
    main: '#512FC9',
    accent: '#651FFF',
    secondaryAccent: '#354AA4',
  },

  text: {
    // The most important text.
    primary: 'rgba(255,255,255,0.87)',
    // Secondary text.
    secondary: 'rgba(255, 255, 255, 0.56)',
    // Placeholder text for forms.
    placeholder: 'rgba(255, 255, 255, 0.24)',
    // Disabled text have even lower visual prominence.
    disabled: 'rgba(0, 0, 0, 0.24)',
    // For maximum contrast.
    contrast: '#FFFFFF',
    // For text on  a background that is on a color opposite to the theme. For dark theme,
    // this would mean text that is on a light background.
    primaryInverse: '#324148',
  },

  buttons: {
    text: 'rgba(255,255,255,0.87)',
    textDisabled: 'rgba(255, 255, 255, 0.3)',
    bgDisabled: 'rgba(255, 255, 255, 0.12)',

    primary: {
      default: '#512FC9',
      hover: '#651FFF',
      active: '#354AA4',
    },

    secondary: {
      default: '#222C59',
      hover: '#2C3A73',
    },

    border: {
      default: '#2C3A73',
      hover: '#2C3A73',
      border: '#1C254D',
      borderHover: 'rgba(255, 255, 255, 0.1)',
    },

    warning: {
      default: '#d50000',
      hover: '#ff1744',
    },

    outlinedPrimary: {
      text: '#651FFF',
      border: '#512FC9',
      borderHover: '#651FFF',
      borderActive: '#354AA4',
    },

    outlinedDefault: {
      text: 'rgba(255,255,255,0.87)',
      textHover: '#FFFFFF',
      border: 'rgba(255,255,255,0.87)',
      borderHover: '#FFFFFF',
    },

    trashButton: {
      default: '#2e3860',
      hover: '#414b70',
    },
  },

  progressBarColor: '#00BFA5',

  dark: '#000000',
  light: '#FFFFFF',

  grey: {
    ...blueGrey,
  },

  error: {
    light: red['A200'],
    dark: red['A700'],
    main: red['A400'],
  },

  action: {
    active: '#FFFFFF',
    hover: 'rgba(255, 255, 255, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(255, 255, 255, 0.2)',
    disabled: 'rgba(255, 255, 255, 0.3)',
    disabledBackground: 'rgba(255, 255, 255, 0.12)',
  },

  subtle: blueGrey[50],
  link: lightBlue[500],
  bgTerminal: '#010B1C',
  danger: pink.A400,
  highlight: yellow[50],
  disabled: blueGrey[500],
  info: lightBlue[600],
  warning: orange.A400,
  success: teal.A700,
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
