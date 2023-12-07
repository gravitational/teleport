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

import { darken } from '../utils/colorManipulator';
import { blue } from '../palette';

import { sharedColors, sharedStyles } from './sharedStyles';
import { DataVisualisationColors, Theme, ThemeColors } from './types';

const dataVisualisationColors: DataVisualisationColors = {
  primary: {
    purple: '#5531d4',
    wednesdays: '#a70daf',
    picton: '#006bb8',
    sunflower: '#8f5f00',
    caribbean: '#007562',
    abbey: '#bf372e',
    cyan: '#007282',
  },
  secondary: {
    purple: '#6f4ced',
    wednesdays: '#dc37e5',
    picton: '#0089de',
    sunflower: '#b27800',
    caribbean: '#009681',
    abbey: '#d4635b',
    cyan: '#1792a3',
  },
  tertiary: {
    purple: '#3d1bb2',
    wednesdays: '#690274',
    picton: '#004b89',
    sunflower: '#704b00',
    caribbean: '#005742',
    abbey: '#9d0a00',
    cyan: '#015c6e',
  },
};

const levels = {
  deep: '#e6e9ea',

  sunken: '#f1f2f4',

  surface: '#fbfbfc',

  elevated: '#ffffff',

  popout: '#ffffff',
};

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: ['rgba(0,0,0,0.06)', 'rgba(0,0,0,0.13)', 'rgba(0,0,0,0.18)'],

  brand: '#512fc9',

  interactive: {
    tonal: {
      primary: [
        'rgba(81,47,201, 0.1)',
        'rgba(81,47,201, 0.18)',
        'rgba(81,47,201, 0.25)',
      ],
    },
  },

  text: {
    main: '#000000',
    slightlyMuted: 'rgba(0,0,0,0.72)',
    muted: 'rgba(0,0,0,0.54)',
    disabled: 'rgba(0,0,0,0.36)',
    primaryInverse: '#ffffff',
  },

  buttons: {
    text: '#000000',
    textDisabled: 'rgba(0,0,0,0.3)',
    bgDisabled: 'rgba(0,0,0,0.12)',

    primary: {
      text: '#ffffff',
      default: '#512fc9',
      hover: '#4126a1',
      active: '#311c79',
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
      text: '#ffffff',
      default: '#cc372d',
      hover: '#a32c24',
      active: '#7a211b',
    },

    trashButton: {
      default: 'rgba(0,0,0,0.07)',
      hover: 'rgba(0,0,0,0.13)',
    },

    link: {
      default: '#0073ba',
      hover: '#005c95',
      active: '#004570',
    },
  },

  tooltip: {
    background: '#f0f2f4',
  },

  progressBarColor: '#007d6b',

  error: {
    main: '#cc372d',
    hover: '#a32c24',
    active: '#7a211b',
  },

  warning: {
    main: '#ffab00',
    hover: '#cc8900',
    active: '#996700',
  },

  notice: {
    background: blue[50],
  },

  action: {
    active: '#ffffff',
    hover: 'rgba(255, 255, 255, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(255, 255, 255, 0.2)',
    disabled: 'rgba(255, 255, 255, 0.3)',
    disabledBackground: 'rgba(255, 255, 255, 0.12)',
  },

  terminal: {
    foreground: '#000000',
    background: levels.sunken,
    selectionBackground: 'rgba(0, 0, 0, 0.18)',
    cursor: '#000000',
    cursorAccent: levels.sunken,
    red: dataVisualisationColors.tertiary.abbey,
    green: dataVisualisationColors.tertiary.caribbean,
    yellow: dataVisualisationColors.tertiary.sunflower,
    blue: dataVisualisationColors.tertiary.picton,
    magenta: dataVisualisationColors.tertiary.purple,
    cyan: dataVisualisationColors.tertiary.cyan,
    brightWhite: darken(levels.sunken, 0.55),
    white: darken(levels.sunken, 0.68),
    brightBlack: darken(levels.sunken, 0.8),
    black: '#000000',
    brightRed: dataVisualisationColors.primary.abbey,
    brightGreen: dataVisualisationColors.primary.caribbean,
    brightYellow: dataVisualisationColors.primary.sunflower,
    brightBlue: dataVisualisationColors.primary.picton,
    brightMagenta: dataVisualisationColors.primary.purple,
    brightCyan: dataVisualisationColors.primary.cyan,
  },

  editor: {
    abbey: dataVisualisationColors.primary.abbey,
    purple: dataVisualisationColors.primary.purple,
    cyan: dataVisualisationColors.primary.cyan,
    picton: dataVisualisationColors.primary.picton,
    sunflower: dataVisualisationColors.primary.sunflower,
    caribbean: dataVisualisationColors.primary.caribbean,
  },

  link: '#0073ba',
  success: '#007d6b',

  dataVisualisation: dataVisualisationColors,
};

const theme: Theme = {
  ...sharedStyles,
  name: 'light',
  type: 'light',
  isCustomTheme: false,
  colors,
};

export default theme;
