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

  sunken: '#141414',

  surface: '#1C1C1C',

  elevated: '#242424',

  popout: '#2C2C2C',
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

  brand: '#049FD9',

  interactive: {
    solid: {
      primary: {
        default: '#049FD9',
        hover: '#1AAEE0',
        active: '#0385B5',
      },
      success: {
        default: '#00A223',
        hover: '#35D655',
        active: '#00961E',
      },
      accent: {
        default: '#5BC8F5',
        hover: '#7DD5F7',
        active: '#3AB5E3',
      },
      danger: {
        default: '#E51E3C',
        hover: '#FD2D4A',
        active: '#E52840',
      },
      alert: {
        default: '#FA5A28',
        hover: '#FB754C',
        active: '#D64D22',
      },
    },

    tonal: {
      primary: [
        'rgba(4, 159, 217, 0.1)',
        'rgba(4, 159, 217, 0.18)',
        'rgba(4, 159, 217, 0.25)',
      ],
      success: [
        'rgba(0, 162, 35, 0.1)',
        'rgba(0, 162, 35, 0.18)',
        'rgba(0, 162, 35, 0.25)',
      ],
      danger: [
        'rgba(229, 30, 60, 0.1)',
        'rgba(229, 30, 60, 0.18)',
        'rgba(229, 30, 60, 0.25)',
      ],
      alert: [
        'rgba(250, 90, 40, 0.1)',
        'rgba(250, 90, 40, 0.18)',
        'rgba(250, 90, 40, 0.25)',
      ],
      informational: [
        'rgba(91, 200, 245, 0.1)',
        'rgba(91, 200, 245, 0.18)',
        'rgba(91, 200, 245, 0.25)',
      ],
      neutral: [neutralColors[0], neutralColors[1], neutralColors[2]],
    },
  },

  text: {
    main: '#FFFFFF',
    slightlyMuted: '#C8C8C8',
    muted: '#8C8C8C',
    disabled: '#606060',
    primaryInverse: '#000000',
  },

  buttons: {
    text: '#FFFFFF',
    textDisabled: 'rgba(255, 255, 255, 0.3)',
    bgDisabled: 'rgba(255, 255, 255, 0.12)',

    primary: {
      text: '#000000',
      default: '#049FD9',
      hover: '#1AAEE0',
      active: '#0385B5',
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
      text: '#FFFFFF',
      default: '#E51E3C',
      hover: '#FD2D4A',
      active: '#E52840',
    },

    trashButton: {
      default: 'rgba(255, 255, 255, 0.07)',
      hover: 'rgba(255, 255, 255, 0.13)',
    },

    link: {
      default: '#049FD9',
      hover: '#1AAEE0',
      active: '#0B8EC2',
    },
  },

  tooltip: {
    background: 'rgba(255, 255, 255, 0.8)',
    inverseBackground: 'rgba(0, 0, 0, 0.5)',
    inverseLinkDefault: '#0073BA',
  },

  progressBarColor: '#049FD9',

  success: {
    main: '#00A223',
    hover: '#35D655',
    active: '#00961E',
  },

  error: {
    main: '#E51E3C',
    hover: '#FD2D4A',
    active: '#E52840',
  },

  warning: {
    main: '#FA5A28',
    hover: '#FB754C',
    active: '#D64D22',
  },

  accent: {
    main: 'rgba(4, 159, 217, 1)',
    hover: 'rgba(26, 174, 224, 1)',
    active: 'rgba(3, 133, 181, 1)',
  },

  notice: {
    background: '#242424', // elevated
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
    selectionBackground: 'rgba(4, 159, 217, 0.25)',
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
    searchMatch: '#A3DCF5',
    activeSearchMatch: '#049FD9',
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
    resource: '#5BC8F5',
    user: '#C5B6FF',
    player: {
      progressBar: {
        background: 'rgba(255, 255, 255, 0.2)',
        seeking: 'rgba(255, 255, 255, 0.17)',
        progress: '#049FD9',
      },
    },
    riskLevels: {
      low: '#00A223',
      medium: '#FA5A28',
      high: '#FD2D4A',
      critical: '#E51E3C',
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
        background: 'rgba(4, 159, 217, 0.25)',
        text: 'rgba(255, 255, 255, 0.6)',
      },
      resize: {
        semiBackground: 'rgba(0, 0, 0, 0.8)',
        background: '#26323c',
        border: '#86c4ed',
        text: '#86c4ed',
      },
      join: {
        background: '#049FD9',
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

  link: '#049FD9',

  highlightedNavigationItem: 'rgba(4, 159, 217, 0.2)',

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
