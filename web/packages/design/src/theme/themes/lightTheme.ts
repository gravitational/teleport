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

import { blue } from '../palette';
import { darken } from '../utils/colorManipulator';
import { sharedColors, sharedStyles } from './sharedStyles';
import { DataVisualisationColors, Theme, ThemeColors } from './types';

const dataVisualisationColors: DataVisualisationColors = {
  primary: {
    purple: '#5531D4',
    wednesdays: '#A70DAF',
    picton: '#006BB8',
    sunflower: '#8F5F00',
    caribbean: '#007562',
    abbey: '#BF372E',
    cyan: '#007282',
  },
  secondary: {
    purple: '#6F4CED',
    wednesdays: '#DC37E5',
    picton: '#0089DE',
    sunflower: '#B27800',
    caribbean: '#009681',
    abbey: '#D4635B',
    cyan: '#1792A3',
  },
  tertiary: {
    purple: '#3D1BB2',
    wednesdays: '#690274',
    picton: '#004B89',
    sunflower: '#704B00',
    caribbean: '#005742',
    abbey: '#9D0A00',
    cyan: '#015C6E',
  },
};

const levels = {
  deep: '#E6E9EA',

  sunken: '#F1F2F4',

  surface: '#FBFBFC',

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

  brand: '#512FC9',

  interactive: {
    solid: {
      primary: {
        default: '#512FC9',
        hover: '#4126A1',
        active: '#311C79',
      },
      success: {
        default: '#007D6B',
        hover: '#006456',
        active: '#004B40',
      },
      accent: {
        default: '#0073BA',
        hover: '#005C95',
        active: '#004570',
      },
      danger: {
        default: '#CC372D',
        hover: '#A32C24',
        active: '#7A211B',
      },
      alert: {
        default: '#FFAB00',
        hover: '#CC8900',
        active: '#996700',
      },
    },

    tonal: {
      primary: [
        'rgba(81,47,201, 0.1)',
        'rgba(81,47,201, 0.18)',
        'rgba(81,47,201, 0.25)',
      ],
      success: [
        'rgba(0, 125, 107, 0.1)',
        'rgba(0, 125, 107, 0.18)',
        'rgba(0, 125, 107, 0.25)',
      ],
      danger: [
        'rgba(204, 55, 45, 0.1)',
        'rgba(204, 55, 45, 0.18)',
        'rgba(204, 55, 45, 0.25)',
      ],
      alert: [
        'rgba(255, 171, 0, 0.1)',
        'rgba(255, 171, 0, 0.18)',
        'rgba(255, 171, 0, 0.25)',
      ],
      informational: [
        'rgba(0, 115, 186, 0.1)',
        'rgba(0, 115, 186, 0.18)',
        'rgba(0, 115, 186, 0.25)',
      ],
      neutral: [neutralColors[0], neutralColors[1], neutralColors[2]],
    },
  },

  text: {
    main: '#000000',
    slightlyMuted: 'rgba(0,0,0,0.72)',
    muted: 'rgba(0,0,0,0.54)',
    disabled: 'rgba(0,0,0,0.36)',
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
    background: 'rgba(0, 0, 0, 0.80)',
    inverseBackground: 'rgba(255, 255, 255, 0.5)',
    inverseLinkDefault: '#009EFF',
  },

  progressBarColor: '#007D6B',

  success: {
    main: '#007D6B',
    hover: '#006456',
    active: '#004B40',
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

  accent: {
    main: 'rgba(0, 115, 186, 1)',
    hover: 'rgba(0, 92, 149, 1)',
    active: 'rgba(0, 69, 112, 1)',
  },

  notice: {
    background: blue[50],
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
    background: levels.sunken,
    selectionBackground: 'rgba(0, 0, 0, 0.18)',
    cursor: '#000',
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
    searchMatch: '#FFD98C',
    activeSearchMatch: '#FFAB00',
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
    resource: '#004570',
    user: '#311C79',
    player: {
      progressBar: {
        background: 'rgba(0, 0, 0, 0.1)',
        seeking: 'rgba(0, 0, 0, 0.15)',
        progress: '#9F85FF',
      },
    },
    riskLevels: {
      low: '#007D6B',
      medium: '#FFAB00',
      high: '#CC372D',
      critical: '#A32C24',
    },
  },

  sessionRecordingTimeline: {
    background: levels.deep,
    headerBackground: 'rgba(0, 0, 0, 0.05)',
    frameBorder: 'rgba(0, 0, 0, 0.2)',
    progressLine: '#E53E3E',
    border: {
      default: '#9a9dab',
      hover: '#5f659e',
    },
    cursor: 'rgba(0, 0, 0, 0.4)',
    events: {
      inactivity: {
        background: 'rgba(81,47,201, 0.25)',
        text: 'rgba(0, 0, 0, 0.6)',
      },
      resize: {
        semiBackground: 'rgba(0, 0, 0, 0.8)',
        border: '#26323c',
        background: '#86c4ed',
        text: '#26323c',
      },
      join: {
        background: '#0073BA',
        text: 'rgba(255, 255, 255, 0.8)',
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

  link: '#0073BA',

  highlightedNavigationItem: blue[200],

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
