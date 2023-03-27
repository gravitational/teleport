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

const colors = {
  // missing
  inverse: '#B0BEC5',
  progressBarColor: '#00BFA5',

  accent: '#651FFF',

  dark: '#000',

  light: '#FFFFFF',

  primary: {
    darker: '#0C143D',
    dark: '#131B43',
    main: '#182047',
    light: '#222C59',
    lighter: '#2D3761',
    contrastText: '#FFFFFF',
  },

  secondary: {
    main: '#512FC9',
    light: '#651FFF',
    dark: '#354AA4',
    contrastText: '#FFFFFF',
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
    // Text hints.
    hint: 'rgba(0, 0, 0, 0.24)',
    // On light backgrounds
    onLight: '#324148',
    // On dark backgrounds
    onDark: 'rgba(255, 255, 255, 0.87)',
  },

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

  subtle: '#CFD8DC',
  link: '#039BE5',

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

const sansSerif = 'system-ui';

const theme = {
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
