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
import { getContrastRatio, lighten } from './utils/colorManipulator';
import { blueGrey, lightBlue, yellow } from './palette';
import typography, { fontSizes, fontWeights } from './typography';
import { sharedStyles } from './sharedStyles';

const space = [0, 4, 8, 16, 24, 32, 40, 48, 56, 64, 72, 80];
const contrastThreshold = 3;

const dataVisualisationColors = {
  primary: {
    purple: '#9F85FF',
    wednesdays: '#F74DFF',
    picton: '#009EFF',
    sunflower: '#FFAB00',
    caribbean: '#00BFA6',
    abbey: '#FF6257',
    cyan: '#00D3F0',
  },
  secondary: {
    purple: '#7D59FF',
    wednesdays: '#D50DE0',
    picton: '#007CC9',
    sunflower: '#AC7400',
    caribbean: '#008775',
    abbey: '#DB3F34',
    cyan: '#009CB1',
  },
  tertiary: {
    purple: '#B9A6FF',
    wednesdays: '#FA96FF',
    picton: '#7BCDFF',
    sunflower: '#FFD98C',
    caribbean: '#2EFFD5',
    abbey: '#FF948D',
    cyan: '#74EEFF',
  },
};

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
    deep: '#000000',

    sunken: '#0C143D',

    surface: '#222C59',

    elevated: '#344179',

    popout: '#4A5688',
  },

  // Spot backgrounds are used as highlights, for example
  // to indicate a hover or active state for an item in a menu.
  spotBackground: [
    'rgba(255,255,255,0.07)',
    'rgba(255,255,255,0.13)',
    'rgba(255,255,255,0.18)',
  ],

  brand: '#9F85FF',

  text: {
    // The most important text.
    main: '#FFFFFF',
    // Slightly muted text.
    slightlyMuted: 'rgba(255, 255, 255, 0.72)',
    // Muted text. Also used as placeholder text in forms.
    muted: 'rgba(255, 255, 255, 0.54)',
    // Disabled text.
    disabled: 'rgba(255, 255, 255, 0.36)',
    // For text on  a background that is on a color opposite to the theme. For dark theme,
    // this would mean text that is on a light background.
    primaryInverse: '#000000',
  },

  buttons: {
    text: '#FFFFFF',
    textDisabled: 'rgba(255, 255, 255, 0.3)',
    bgDisabled: 'rgba(255, 255, 255, 0.12)',

    primary: {
      text: '#000000',
      default: '#9F85FF',
      hover: '#B29DFF',
      active: '#C5B6FF',
    },

    secondary: {
      default: 'rgba(255,255,255,0.07)',
      hover: 'rgba(255,255,255,0.13)',
      active: 'rgba(255,255,255,0.18)',
    },

    border: {
      default: 'rgba(255,255,255,0)',
      hover: 'rgba(255, 255, 255, 0.07)',
      active: 'rgba(255, 255, 255, 0.13)',
      border: 'rgba(255, 255, 255, 0.36)',
    },

    warning: {
      text: '#000000',
      default: '#FF6257',
      hover: '#FF8179',
      active: '#FFA19A',
    },

    trashButton: {
      default: 'rgba(255, 255, 255, 0.07)',
      hover: 'rgba(255, 255, 255, 0.13)',
    },

    link: {
      default: '#009EFF',
      hover: '#33B1FF',
      active: '#66C5FF',
    },
  },

  tooltip: {
    background: '#212B2F',
  },

  progressBarColor: '#00BFA5',

  dark: '#000000',
  light: '#FFFFFF',

  grey: {
    ...blueGrey,
  },

  error: {
    main: '#FF6257',
    hover: '#FF8179',
    active: '#FFA19A',
  },

  warning: {
    main: '#FFAB00',
    hover: '#FFBC33',
    active: '#FFCD66',
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
    foreground: '#F1F2F4',
    background: '#0C143D', // sunken
    selectionBackground: 'rgba(255, 255, 255, 0.18)',
    cursor: '#FFF',
    cursorAccent: '#0C143D',
    red: dataVisualisationColors.primary.abbey,
    green: dataVisualisationColors.primary.caribbean,
    yellow: dataVisualisationColors.primary.sunflower,
    blue: dataVisualisationColors.primary.picton,
    magenta: dataVisualisationColors.primary.purple,
    cyan: dataVisualisationColors.primary.cyan,
    brightWhite: lighten('#0C143D', 0.89),
    white: lighten('#0C143D', 0.78),
    brightBlack: lighten('#0C143D', 0.61),
    black: '#000',
    brightRed: dataVisualisationColors.tertiary.abbey,
    brightGreen: dataVisualisationColors.tertiary.caribbean,
    brightYellow: dataVisualisationColors.tertiary.sunflower,
    brightBlue: dataVisualisationColors.tertiary.picton,
    brightMagenta: dataVisualisationColors.tertiary.purple,
    brightCyan: dataVisualisationColors.tertiary.cyan,
  },

  subtle: blueGrey[50],
  link: '#009EFF',
  bgTerminal: '#010B1C',
  highlight: yellow[50],
  disabled: blueGrey[500],
  info: lightBlue[600],
  success: '#00BFA5',

  dataVisualisation: dataVisualisationColors,
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
  name: 'dark',
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
