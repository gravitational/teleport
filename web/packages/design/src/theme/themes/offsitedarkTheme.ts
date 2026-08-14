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
    purple: '#B587FA',
    wednesdays: '#F26DD1',
    picton: '#8A95FF',
    sunflower: '#F7782F',
    caribbean: '#36B26E',
    abbey: '#F57398',
    cyan: '#17C2C2',
  },
  secondary: {
    purple: '#8D4EED',
    wednesdays: '#A55B96',
    picton: '#6971AC',
    sunflower: '#C44F14',
    caribbean: '#38835C',
    abbey: '#CF3A7A',
    cyan: '#0BB2B8',
  },
  tertiary: {
    purple: '#C299FF',
    wednesdays: '#F582D8',
    picton: '#9CA6FF',
    sunflower: '#FC8D4C',
    caribbean: '#4CBF7F',
    abbey: '#FF87A9',
    cyan: '#4AD9D9',
  },
};

const levels = {
  deep: '#000000',

  sunken: '#0F1214',

  surface: '#23282E',

  elevated: '#373C42',

  popout: '#464C54',
};

const neutralColors = [
  'rgba(167, 173, 181, 0.07)',
  'rgba(167, 173, 181, 0.13)',
  'rgba(167, 173, 181, 0.18)',
];

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: neutralColors,

  brand: '#649EF5',

  interactive: {
    solid: {
      primary: {
        default: '#649EF5',
        hover: '#83B1F7',
        active: '#A2C5F9',
      },
      success: {
        default: '#33BBF5',
        hover: '#5CC9F7',
        active: '#85D6F9',
      },
      accent: {
        default: '#649EF5',
        hover: '#83B1F7',
        active: '#A2C5F9',
      },
      danger: {
        default: '#FA5762',
        hover: '#FB7981',
        active: '#FC9AA1  ',
      },
      alert: {
        default: '#F0C243',
        hover: '#F3CE69',
        active: '#F6DA8E',
      },
    },

    tonal: {
      primary: [
        'rgba(100, 158, 245, 0.1)',
        'rgba(100, 158, 245, 0.18)',
        'rgba(100, 158, 245, 0.25)',
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
    muted: '#9ba4ae',
    disabled: '#6F7680',
    primaryInverse: '#23282E',
  },

  buttons: {
    text: '#F7F7F7',
    textDisabled: '#6F7680',
    bgDisabled: '#464C54',

    primary: {
      text: '#23282E',
      default: '#649EF5',
      hover: '#83B1F7',
      active: '#A2C5F9',
    },

    secondary: {
      default: neutralColors[0],
      hover: neutralColors[1],
      active: neutralColors[2],
    },

    border: {
      default: 'rgba(255,255,255,0)',
      hover: neutralColors[0],
      active: neutralColors[1],
      border: 'rgba(255, 255, 255, 0.36)',
    },

    warning: {
      text: '#23282E',
      default: '#FA5762',
      hover: '#FB7981',
      active: '#FC9AA1',
    },

    trashButton: {
      default: neutralColors[0],
      hover: neutralColors[1],
    },

    link: {
      default: '#649EF5',
      hover: '#83B1F7',
      active: '#A2C5F9',
    },
  },

  tooltip: {
    background: 'rgba(255, 255, 255, 0.8)',
    inverseBackground: 'rgba(0, 0, 0, 0.5)',
    inverseLinkDefault: '#649EF5',
  },

  progressBarColor: '#52A62B',

  success: {
    main: '#33BBF5',
    hover: '#5CC9F7',
    active: '#85D6F9',
  },

  error: {
    main: '#FA5762',
    hover: '#FB7981',
    active: '#FC9AA1',
  },

  warning: {
    main: '#F0C243',
    hover: '#F3CE69',
    active: '#F6DA8E',
  },

  accent: {
    main: '#649ef5',
    hover: '#83B1F7',
    active: '#A2C5F9',
  },

  notice: {
    background: '#4a576d',
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
    selectionBackground: 'rgba(100, 158, 245, 0.25)',
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
    searchMatch: '#195385',
    activeSearchMatch: '#33BBF5',
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
    user: '#33BBF5',
    player: {
      progressBar: {
        background: 'rgba(255, 255, 255, 0.2)',
        seeking: 'rgba(255, 255, 255, 0.17)',
        progress: '#33BBF5',
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
    background: levels.sunken,
    headerBackground: 'rgba(0, 0, 0, 0.13)',
    frameBorder: 'rgba(255, 255, 255, 0.2)',
    progressLine: dataVisualisationColors.primary.abbey,
    border: {
      default: '#3A4A5A',
      hover: '#5A7A9A',
    },
    cursor: 'rgba(255, 255, 255, 0.4)',
    events: {
      inactivity: {
        background: 'rgba(100, 158, 245, 0.25)',
        text: 'rgba(255, 255, 255, 0.6)',
      },
      resize: {
        semiBackground: 'rgba(0, 0, 0, 0.8)',
        background: '#26323c',
        border: '#D0D4D9',
        text: '#D0D4D9',
      },
      join: {
        background: '#4d7bbf',
        text: 'rgba(0, 0, 0, 0.87)',
      },
      default: {
        background: 'rgba(255, 255, 255, 0.54)',
        text: '#FFFFFF',
      },
    },
    timeMarks: {
      primary: 'rgba(255, 255,255, 0.54)',
      secondary: 'rgba(0255, 255, 255, 0.36)',
      absolute: 'rgba(255, 255, 255, 0.87)',
      text: 'rgba(255, 255, 255, 0.87)',
    },
  },

  link: '#649EF5',

  highlightedNavigationItem: 'rgba(100, 158, 245, 0.2)',

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
