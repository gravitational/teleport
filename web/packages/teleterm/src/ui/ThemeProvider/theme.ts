/*
Copyright 2019 Gravitational, Inc.

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

import { fonts } from 'design/theme/fonts';
import { getContrastRatio } from 'design/theme/utils/colorManipulator';
import {
  lightBlue,
  red,
  teal,
  orange,
  pink,
  blueGrey,
  yellow,
} from 'design/theme/palette';
import typography, { fontSizes, fontWeights } from 'design/theme/typography';

const space = [0, 4, 8, 16, 24, 32, 40, 48, 56, 64, 72, 80];
const contrastThreshold = 3;

/*
  Fields marked with "Not in v13+" are color fields that are not included in the themes released in v13 and onwards.
  Fields marked with "Only in use in v13+" are color fields that are not used by any components in this version, but are included here in order to make
  backporting components from later versions easier.
*/
const colors = {
  levels: {
    sunken: '#0C143D',
    sunkenSecondary: '#131B43', // Not in v13+.

    surface: '#222C59',
    surfaceSecondary: '#182047', // Not in v13+.

    elevated: '#2D3761',
  },

  // Spot backgrounds are used as highlights, for example
  // to indicate a hover or active state for an item in a menu.
  spotBackground: [
    'rgba(255,255,255,0.07)',
    'rgba(255,255,255,0.13)',
    'rgba(255,255,255,0.18)',
  ], // Only in use in v13+.

  brand: '#512FC9',
  brandAccent: '#651FFF', // Not in v13+.
  brandSecondaryAccent: '#354AA4', // Not in v13+.

  text: {
    // The most important text.
    main: 'rgba(255,255,255,0.87)',
    // Slightly muted text.
    slightlyMuted: 'rgba(255, 255, 255, 0.72)',
    // Muted text. Also used as placeholder text in forms.
    muted: 'rgba(255, 255, 255, 0.54)',
    // Disabled text.
    disabled: 'rgba(255, 255, 255, 0.36)',
    // For text on  a background that is on a color opposite to the theme. For dark theme,
    // this would mean text that is on a light background.
    primaryInverse: '#000000',
    // For maximum contrast.
    contrast: '#FFFFFF', // Not in v13+.
  },

  buttons: {
    text: 'rgba(255,255,255,0.87)',
    textDisabled: 'rgba(255, 255, 255, 0.3)',
    bgDisabled: 'rgba(255, 255, 255, 0.12)',

    primary: {
      text: '#FFFFFF', // Only in use in v13+.
      default: '#512FC9',
      hover: '#651FFF',
      active: '#354AA4',
    },

    secondary: {
      default: '#222C59',
      hover: '#2C3A73',
      active: '#2C3A73',
    },

    border: {
      default: '#2C3A73',
      hover: '#2C3A73',
      border: '#1C254D',
      borderHover: 'rgba(255, 255, 255, 0.1)',
      active: '#2C3A73', // Only in use in v13+.
    },

    warning: {
      default: '#d50000',
      hover: '#ff1744',
      text: '#000000', // Only in use in v13+.
      active: '#ff1744', // Only in use in v13+.
    },

    // Not in v13+.
    outlinedPrimary: {
      text: '#651FFF',
      border: '#512FC9',
      borderHover: '#651FFF',
      borderActive: '#354AA4',
    },

    // Not in v13+.
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

    // Only in use in v13+.
    link: {
      default: '#009EFF',
      hover: '#33B1FF',
      active: '#66C5FF',
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
    hover: red['A200'], // Only in use in v13+.
    active: red['A100'], // Only in use in v13+.
  },

  warning: {
    main: orange['A400'],
    hover: orange['A200'], // Only in use in v13+.
    active: orange['A100'], // Only in use in v13+.
  },

  action: {
    active: '#FFFFFF',
    hover: 'rgba(255, 255, 255, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(255, 255, 255, 0.2)',
    disabled: 'rgba(255, 255, 255, 0.3)',
    disabledBackground: 'rgba(255, 255, 255, 0.12)',
  },

  subtle: '#CFD8DC',
  link: '#039BE5',

  danger: pink.A400,
  highlight: yellow[50],
  disabled: blueGrey[500],
  info: lightBlue[600],
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

const sansSerif = 'system-ui';

const theme = {
  name: 'dark',
  colors,
  typography,
  font: sansSerif,
  fonts: {
    sansSerif,
    mono: fonts.mono,
  },
  fontWeights,
  fontSizes,
  space,
  borders,
  radii: [0, 2, 4, 8, 16, 9999, '100%'],
  regular: fontWeights.regular,
  bold: fontWeights.bold,
  boxShadow: [
    '0px 2px 1px -1px rgba(0, 0, 0, 0.2), 0px 1px 1px rgba(0, 0, 0, 0.14), 0px 1px 3px rgba(0, 0, 0, 0.12)',
    '0px 5px 5px -3px rgba(0, 0, 0, 0.2), 0px 8px 10px 1px rgba(0, 0, 0, 0.14), 0px 3px 14px 2px rgba(0, 0, 0, 0.12)',
    '0px 3px 5px -1px rgba(0, 0, 0, 0.2), 0px 6px 10px rgba(0, 0, 0, 0.14), 0px 1px 18px rgba(0, 0, 0, 0.12)',
  ],
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
