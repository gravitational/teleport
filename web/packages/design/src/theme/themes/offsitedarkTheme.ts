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
    purple: '#9B5FF5',
    wednesdays: '#FF87A9',
    picton: '#9ADFFC',
    sunflower: '#F0C243',
    caribbean: '#169855',
    abbey: '#FA5762',
    cyan: '#17C2C2',
  },
  secondary: {
    purple: '#753BCC',
    wednesdays: '#E3447C',
    picton: '#23A8EB',
    sunflower: '#D6900D',
    caribbean: '#087041',
    abbey: '#CC2D37',
    cyan: '#028E99',
  },
  tertiary: {
    purple: '#D6BAFF',
    wednesdays: '#FFD4E0',
    picton: '#D9F4FF',
    sunflower: '#F5E08E',
    caribbean: '#75D9A0',
    abbey: '#FFB2B5',
    cyan: '#A9EBEB',
  },
};

const levels = {
  deep: '#0F1214',

  sunken: '#0F1214',

  surface: '#23282E',

  elevated: '#373C42',

  popout: '#464C54',
};

const neutralColors = [
  'rgba(255,255,255,0.07)',
  'rgba(255,255,255,0.13)',
  'rgba(255,255,255,0.18)',
];

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: neutralColors,

  brand: '#52A62B',

  interactive: {
    solid: {
      primary: {
        default: '#52A62B',
        hover: '#6BBF41',
        active: '#398519',
      },
      success: {
        default: '#33BBF5',
        hover: '#33BBF5',
        active: '#33BBF5',
      },
      accent: {
        default: '#649EF5',
        hover: '#7CADF7',
        active: '#7CADF7',
      },
      danger: {
        default: '#FA5762',
        hover: '#FF6E72',
        active: '#FF6E72',
      },
      alert: {
        default: '#F0C243',
        hover: '#F0C243',
        active: '#F0C243',
      },
    },

    tonal: {
      primary: [
        'rgba(82, 166, 43, 0.1)',
        'rgba(82, 166, 43, 0.18)',
        'rgba(82, 166, 43, 0.25)',
      ],
      success: [
        'rgba(51, 187, 245, 0.1)',
        'rgba(51, 187, 245, 0.18)',
        'rgba(51, 187, 245, 0.25)',
      ],
      danger: [
        'rgba(250, 87, 98, 0.1)',
        'rgba(250, 87, 98, 0.18)',
        'rgba(250, 87, 98, 0.25)',
      ],
      alert: [
        'rgba(240, 194, 67, 0.1)',
        'rgba(240, 194, 67, 0.18)',
        'rgba(240, 194, 67, 0.25)',
      ],
      informational: [
        'rgba(100, 158, 245, 0.1)',
        'rgba(100, 158, 245, 0.18)',
        'rgba(100, 158, 245, 0.25)',
      ],
      neutral: [neutralColors[0], neutralColors[1], neutralColors[2]],
    },
  },

  text: {
    main: '#F7F7F7',
    slightlyMuted: '#D0D4D9',
    muted: '#889099',
    disabled: '#6F7680',
    primaryInverse: '#23282E',
  },

  buttons: {
    text: '#F7F7F7',
    textDisabled: '#6F7680',
    bgDisabled: '#464C54',

    primary: {
      text: '#23282E',
      default: '#52A62B',
      hover: '#6BBF41',
      active: '#398519',
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
      text: '#23282E',
      default: '#FA5762',
      hover: '#FF6E72',
      active: '#FF6E72',
    },

    trashButton: {
      default: 'rgba(255, 255, 255, 0.07)',
      hover: 'rgba(255, 255, 255, 0.13)',
    },

    link: {
      default: '#649EF5',
      hover: '#7CADF7',
      active: '#7CADF7',
    },
  },

  tooltip: {
    background: '#464C54',
    inverseBackground: 'rgba(0, 0, 0, 0.5)',
    inverseLinkDefault: '#649EF5',
  },

  progressBarColor: '#52A62B',

  success: {
    main: '#33BBF5',
    hover: '#33BBF5',
    active: '#33BBF5',
  },

  error: {
    main: '#FA5762',
    hover: '#FF6E72',
    active: '#FF6E72',
  },

  warning: {
    main: '#F0C243',
    hover: '#F0C243',
    active: '#F0C243',
  },

  accent: {
    main: 'rgba(100, 158, 245, 1)',
    hover: 'rgba(124, 173, 247, 1)',
    active: 'rgba(124, 173, 247, 1)',
  },

  notice: {
    background: '#534A6D',
  },

  action: {
    active: '#F7F7F7',
    hover: 'rgba(255, 255, 255, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(255, 255, 255, 0.2)',
    disabled: '#6F7680',
    disabledBackground: '#464C54',
  },

  terminal: {
    foreground: '#F7F7F7',
    background: levels.sunken,
    selectionBackground: 'rgba(82, 166, 43, 0.25)',
    cursor: '#F7F7F7',
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
    searchMatch: '#398519',
    activeSearchMatch: '#52A62B',
  },

  editor: {
    abbey: dataVisualisationColors.tertiary.abbey,
    purple: dataVisualisationColors.tertiary.purple,
    cyan: dataVisualisationColors.tertiary.cyan,
    picton: dataVisualisationColors.tertiary.picton,
    sunflower: dataVisualisationColors.tertiary.sunflower,
    caribbean: dataVisualisationColors.tertiary.caribbean,
  },

  sessionRecording: {
    resource: '#649EF5',
    user: '#52A62B',
    player: {
      progressBar: {
        background: 'rgba(255, 255, 255, 0.2)',
        seeking: 'rgba(255, 255, 255, 0.17)',
        progress: '#52A62B',
      },
    },
    riskLevels: {
      low: '#33BBF5',
      medium: '#F0C243',
      high: '#F7782F',
      critical: '#FA5762',
    },
  },

  sessionRecordingTimeline: {
    background: levels.sunken,
    headerBackground: 'rgba(0, 0, 0, 0.13)',
    frameBorder: 'rgba(255, 255, 255, 0.2)',
    progressLine: '#E53E3E',
    border: {
      default: '#3A4A5A',
      hover: '#5A7A9A',
    },
    cursor: 'rgba(255, 255, 255, 0.4)',
    events: {
      inactivity: {
        background: 'rgba(82, 166, 43, 0.25)',
        text: 'rgba(255, 255, 255, 0.6)',
      },
      resize: {
        semiBackground: 'rgba(0, 0, 0, 0.8)',
        background: '#26323c',
        border: '#86c4ed',
        text: '#86c4ed',
      },
      join: {
        background: '#649EF5',
        text: 'rgba(0, 0, 0, 0.87)',
      },
      default: {
        background: 'rgba(255, 255, 255, 0.54)',
        text: '',
      },
    },
    timeMarks: {
      primary: '#E2E8F0',
      secondary: '#A0AEC0',
      absolute: '#E2E8F0',
      text: '#A0AEC0',
    },
  },

  link: '#649EF5',

  highlightedNavigationItem: 'rgba(82, 166, 43, 0.2)',

  dataVisualisation: dataVisualisationColors,
};

const theme: Theme = {
  ...sharedStyles,
  name: 'offsitedark',
  type: 'dark',
  isCustomTheme: true,
  colors,
};

export default theme;
