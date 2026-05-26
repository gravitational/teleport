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
import { sharedColors, sharedStyles } from './sharedStyles';
import { DataVisualisationColors, Theme, ThemeColors } from './types';

const dataVisualisationColors: DataVisualisationColors = {
  primary: {
    purple: '#864AE0',
    wednesdays: '#F2638C',
    picton: '#139BEB',
    sunflower: '#CC8604',
    caribbean: '#0B7B46',
    abbey: '#CC2D37',
    cyan: '#04A4B0',
  },
  secondary: {
    purple: '#9B5FF5',
    wednesdays: '#FF87A9',
    picton: '#33BBF5',
    sunflower: '#E0A419',
    caribbean: '#169855',
    abbey: '#EB4651',
    cyan: '#17C2C2',
  },
  tertiary: {
    purple: '#6732B8',
    wednesdays: '#CF3A7A',
    picton: '#087ABD',
    sunflower: '#B05F04',
    caribbean: '#075E39',
    abbey: '#A01D26',
    cyan: '#01818C',
  },
};

const levels = {
  deep: '#F0F1F2',

  sunken: '#F7F7F7',

  surface: '#FFFFFF',

  elevated: '#FFFFFF',

  popout: '#FFFFFF',
};

const neutralColors = [
  'rgba(0,0,0,0.06)',
  'rgba(0,0,0,0.13)',
  'rgba(0,0,0,0.18)',
];

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: neutralColors,

  brand: '#45991F',

  interactive: {
    solid: {
      primary: {
        default: '#1D69CC',
        hover: '#0D5CBD',
        active: '#0D5CBD',
      },
      success: {
        default: '#0570AD',
        hover: '#0570AD',
        active: '#0570AD',
      },
      accent: {
        default: '#2774D9',
        hover: '#0D5CBD',
        active: '#0D5CBD',
      },
      danger: {
        default: '#CC2D37',
        hover: '#B2242D',
        active: '#B2242D',
      },
      alert: {
        default: '#A65503',
        hover: '#A65503',
        active: '#A65503',
      },
    },

    tonal: {
      primary: [
        '#F0F1F2',
        '#E1E4E8CC',
        '#E1E4E8CC',
      ],
      success: [
        'rgba(19, 155, 235, 0.1)',
        'rgba(19, 155, 235, 0.18)',
        'rgba(19, 155, 235, 0.25)',
      ],
      danger: [
        'rgba(204, 45, 55, 0.1)',
        'rgba(204, 45, 55, 0.18)',
        'rgba(204, 45, 55, 0.25)',
      ],
      alert: [
        'rgba(204, 134, 4, 0.1)',
        'rgba(204, 134, 4, 0.18)',
        'rgba(204, 134, 4, 0.25)',
      ],
      informational: [
        'rgba(39, 116, 217, 0.1)',
        'rgba(39, 116, 217, 0.18)',
        'rgba(39, 116, 217, 0.25)',
      ],
      neutral: [neutralColors[0], neutralColors[1], neutralColors[2]],
    },
  },

  text: {
    main: '#23282E',
    slightlyMuted: '#596069',
    muted: '#889099',
    disabled: '#C1C6CC',
    primaryInverse: '#FFFFFF',
  },

  buttons: {
    text: '#23282E',
    textDisabled: '#C1C6CC',
    bgDisabled: '#F0F1F2',

    primary: {
      text: '#FFFFFF',
      default: '#1D69CC',
      hover: '#0D5CBD',
      active: '#0D5CBD',
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
      default: '#CC2D37',
      hover: '#B2242D',
      active: '#B2242D',
    },

    trashButton: {
      default: 'rgba(0,0,0,0.07)',
      hover: 'rgba(0,0,0,0.13)',
    },

    link: {
      default: '#1D69CC',
      hover: '#0D5CBD',
      active: '#0D5CBD',
    },
  },

  tooltip: {
    background: '#373C42',
    inverseBackground: 'rgba(255, 255, 255, 0.5)',
    inverseLinkDefault: '#1D69CC',
  },

  progressBarColor: '#52A62B',

  success: {
    main: '#0570AD',
    hover: '#0570AD',
    active: '#0570AD',
  },

  error: {
    main: '#CC2D37',
    hover: '#B2242D',
    active: '#B2242D',
  },

  warning: {
    main: '#A65503',
    hover: '#A65503',
    active: '#A65503',
  },

  accent: {
    main: 'rgba(39, 116, 217, 1)',
    hover: 'rgba(13, 92, 189, 1)',
    active: 'rgba(13, 92, 189, 1)',
  },

  notice: {
    background: '#F3EBFF',
  },

  action: {
    active: '#23282E',
    hover: 'rgba(255, 255, 255, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(255, 255, 255, 0.2)',
    disabled: '#C1C6CC',
    disabledBackground: '#F0F1F2',
  },

  terminal: {
    foreground: '#23282E',
    background: levels.sunken,
    selectionBackground: 'rgba(82, 166, 43, 0.25)',
    cursor: '#23282E',
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
    black: '#000',
    brightRed: dataVisualisationColors.primary.abbey,
    brightGreen: dataVisualisationColors.primary.caribbean,
    brightYellow: dataVisualisationColors.primary.sunflower,
    brightBlue: dataVisualisationColors.primary.picton,
    brightMagenta: dataVisualisationColors.primary.purple,
    brightCyan: dataVisualisationColors.primary.cyan,
    searchMatch: '#E0F5D5',
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
    resource: '#2774D9',
    user: '#52A62B',
    player: {
      progressBar: {
        background: 'rgba(0, 0, 0, 0.1)',
        seeking: 'rgba(0, 0, 0, 0.15)',
        progress: '#52A62B',
      },
    },
    riskLevels: {
      low: '#139BEB',
      medium: '#CC8604',
      high: '#F26722',
      critical: '#CC2D37',
    },
  },

  sessionRecordingTimeline: {
    background: levels.deep,
    headerBackground: 'rgba(0, 0, 0, 0.05)',
    frameBorder: 'rgba(0, 0, 0, 0.2)',
    progressLine: '#E53E3E',
    border: {
      default: '#93AB90',
      hover: '#5A8055',
    },
    cursor: 'rgba(0, 0, 0, 0.4)',
    events: {
      inactivity: {
        background: 'rgba(82, 166, 43, 0.25)',
        text: 'rgba(0, 0, 0, 0.6)',
      },
      resize: {
        semiBackground: 'rgba(0, 0, 0, 0.8)',
        border: '#26323c',
        background: '#86c4ed',
        text: '#26323c',
      },
      join: {
        background: '#2774D9',
        text: 'rgba(255, 255, 255, 0.87)',
      },
      default: {
        background: 'rgba(0, 0, 0, 0.54)',
        text: '#000',
      },
    },
    timeMarks: {
      primary: 'rgba(0,0,0,0.54)',
      secondary: 'rgba(0,0,0,0.36)',
      absolute: 'rgba(0,0,0,0.87)',
      text: 'rgba(0,0,0,0.87)',
    },
  },

  link: '#1D69CC',

  highlightedNavigationItem: 'rgba(82, 166, 43, 0.2)',

  dataVisualisation: dataVisualisationColors,
};

const theme: Theme = {
  ...sharedStyles,
  name: 'offsitelight',
  type: 'light',
  isCustomTheme: true,
  colors,
};

export default theme;
