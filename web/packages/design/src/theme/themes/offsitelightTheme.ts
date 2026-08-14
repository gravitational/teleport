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
    purple: '#753BCC',
    wednesdays: '#A73D90',
    picton: '#535ED2',
    sunflower: '#AD3907',
    caribbean: '#37794B',
    abbey: '#B02863',
    cyan: '#006773',
  },
  secondary: {
    purple: '#7E4DD8',
    wednesdays: '#D649B3',
    picton: '#6977F0',
    sunflower: '#AF4A21',
    caribbean: '#169855',
    abbey: '#B33D6E',
    cyan: '#54939A',
  },
  tertiary: {
    purple: '#6732B8',
    wednesdays: '#A62686',
    picton: '#4653C7',
    sunflower: '#942E03',
    caribbean: '#087041',
    abbey: '#991D53',
    cyan: '#005C66',
  },
};

const levels = {
  deep: '#E1E4E8',

  sunken: '#F0F1F2',

  surface: '#f7f7f7',

  elevated: '#FFFFFF',

  popout: '#FFFFFF',
};

const neutralColors = [
  'rgba(101, 108, 117, 0.06)',
  'rgba(101, 108, 117, 0.13)',
  'rgba(101, 108, 117, 0.18)',
];

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: neutralColors,

  brand: '#1D69CC',

  interactive: {
    solid: {
      primary: {
        default: '#1D69CC',
        hover: '#1754A3',
        active: '#113F7A',
      },
      success: {
        default: '#139BEB',
        hover: '#0F7CBC',
        active: '#0B5D8D',
      },
      accent: {
        default: '#1D69CC',
        hover: '#1754A3',
        active: '#113F7A',
      },
      danger: {
        default: '#CC2D37',
        hover: '#A3242C',
        active: '#7A1B21',
      },
      alert: {
        default: '#CC8604',
        hover: '#A36B03',
        active: '#7A5002',
      },
    },

    tonal: {
      primary: [
        'rgba(29, 105, 204, 0.1)',
        'rgba(29, 105, 204, 0.18)',
        'rgba(29, 105, 204, 0.25)',
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
        'rgba(134, 74, 224, 0.1)',
        'rgba(134, 74, 224, 0.18)',
        'rgba(134, 74, 224, 0.25)',
      ],
      neutral: [neutralColors[0], neutralColors[1], neutralColors[2]],
    },
  },

  // TODO - update text colors
  text: {
    main: '#23282e',
    slightlyMuted: '#596069',
    muted: '#596069',
    disabled: '#A7ADB5',
    primaryInverse: '#FFFFFF',
  },

  buttons: {
    text: '#23282E',
    textDisabled: '#C1C6CC',
    bgDisabled: '#F0F1F2',

    primary: {
      text: '#FFFFFF',
      default: '#1D69CC',
      hover: '#1754A3',
      active: '#113F7A',
    },

    secondary: {
      default: neutralColors[0],
      hover: neutralColors[1],
      active: neutralColors[2],
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
      hover: '#A3242C',
      active: '#7A1B21',
    },

    trashButton: {
      default: neutralColors[0],
      hover: neutralColors[1],
    },

    link: {
      default: '#1D69CC',
      hover: '#1754A3',
      active: '#113F7A',
    },
  },

  tooltip: {
    background: 'rgba(0, 0, 0, 0.80)',
    inverseBackground: 'rgba(255, 255, 255, 0.5)',
    inverseLinkDefault: '#1D69CC',
  },

  progressBarColor: '#139BEB',

  success: {
    main: '#139BEB',
    hover: '#0F7CBC',
    active: '#0B5D8D',
  },

  error: {
    main: '#CC2D37',
    hover: '#B2242D',
    active: '#7A1B21',
  },

  warning: {
    main: '#CC8604',
    hover: '#A36B03',
    active: '#7A5002',
  },

  accent: {
    main: '#1D69CC',
    hover: '#1754A3',
    active: '#113F7A',
  },

  notice: {
    background: '#f7f7f7',
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
    brightWhite: '#889099',
    white: '#6F7680',
    brightBlack: '#596069',
    black: '#23282E',
    brightRed: dataVisualisationColors.primary.abbey,
    brightGreen: dataVisualisationColors.primary.caribbean,
    brightYellow: dataVisualisationColors.primary.sunflower,
    brightBlue: dataVisualisationColors.primary.picton,
    brightMagenta: dataVisualisationColors.primary.purple,
    brightCyan: dataVisualisationColors.primary.cyan,
    searchMatch: '#d5e8f5',
    activeSearchMatch: '#139BEB',
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
    resource: '#1D69CC',
    user: '#139BEB',
    player: {
      progressBar: {
        background: 'rgba(0, 0, 0, 0.1)',
        seeking: 'rgba(0, 0, 0, 0.15)',
        progress: '#139BEB',
      },
    },
    riskLevels: {
      low: dataVisualisationColors.tertiary.caribbean,
      medium: dataVisualisationColors.tertiary.sunflower,
      high: dataVisualisationColors.tertiary.purple,
      critical: dataVisualisationColors.tertiary.abbey,
    },
  },

  sessionRecordingTimeline: {
    background: levels.deep,
    headerBackground: 'rgba(0, 0, 0, 0.05)',
    frameBorder: 'rgba(0, 0, 0, 0.2)',
    progressLine: dataVisualisationColors.primary.abbey,
    border: {
      default: '#90a0ab',
      hover: '#535ED2',
    },
    cursor: 'rgba(0, 0, 0, 0.4)',
    events: {
      inactivity: {
        background: 'rgba(19, 155, 235, 0.25)',
        text: 'rgba(0, 0, 0, 0.6)',
      },
      resize: {
        semiBackground: 'rgba(0, 0, 0, 0.8)',
        border: '#23282e',
        background: '#86c4ed',
        text: '#23282e',
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

  highlightedNavigationItem: 'rgba(19, 155, 235, 0.2)',

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
