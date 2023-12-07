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

import { lighten } from '../utils/colorManipulator';

import { sharedColors, sharedStyles } from './sharedStyles';
import { DataVisualisationColors, Theme, ThemeColors } from './types';

const dataVisualisationColors: DataVisualisationColors = {
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

const levels = {
  deep: '#000000',

  sunken: '#0C143D',

  surface: '#222C59',

  elevated: '#344179',

  popout: '#4A5688',
};

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: [
    'rgba(255,255,255,0.07)',
    'rgba(255,255,255,0.13)',
    'rgba(255,255,255,0.18)',
  ],

  brand: '#9F85FF',

  interactive: {
    tonal: {
      primary: [
        'rgba(159,133,255, 0.1)',
        'rgba(159,133,255, 0.18)',
        'rgba(159,133,255, 0.25)',
      ],
    },
  },

  text: {
    main: '#FFFFFF',
    slightlyMuted: 'rgba(255, 255, 255, 0.72)',
    muted: 'rgba(255, 255, 255, 0.54)',
    disabled: 'rgba(255, 255, 255, 0.36)',
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

  notice: {
    background: '#344179', // elevated
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
    foreground: '#FFF',
    background: levels.sunken,
    selectionBackground: 'rgba(255, 255, 255, 0.18)',
    cursor: '#FFF',
    cursorAccent: levels.sunken,
    red: dataVisualisationColors.primary.abbey,
    green: dataVisualisationColors.primary.caribbean,
    yellow: dataVisualisationColors.primary.sunflower,
    blue: dataVisualisationColors.primary.picton,
    magenta: dataVisualisationColors.primary.purple,
    cyan: dataVisualisationColors.primary.cyan,
    brightWhite: lighten(levels.sunken, 0.89),
    white: lighten(levels.sunken, 0.78),
    brightBlack: lighten(levels.sunken, 0.61),
    black: '#000',
    brightRed: dataVisualisationColors.tertiary.abbey,
    brightGreen: dataVisualisationColors.tertiary.caribbean,
    brightYellow: dataVisualisationColors.tertiary.sunflower,
    brightBlue: dataVisualisationColors.tertiary.picton,
    brightMagenta: dataVisualisationColors.tertiary.purple,
    brightCyan: dataVisualisationColors.tertiary.cyan,
  },

  editor: {
    abbey: dataVisualisationColors.tertiary.abbey,
    purple: dataVisualisationColors.tertiary.purple,
    cyan: dataVisualisationColors.tertiary.cyan,
    picton: dataVisualisationColors.tertiary.picton,
    sunflower: dataVisualisationColors.tertiary.sunflower,
    caribbean: dataVisualisationColors.tertiary.caribbean,
  },

  link: '#009EFF',
  success: '#00BFA5',

  dataVisualisation: dataVisualisationColors,
};

const theme: Theme = {
  ...sharedStyles,
  name: 'dark',
  type: 'dark',
  isCustomTheme: false,
  colors,
};

export default theme;
