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

import { darken, lighten } from '../utils/colorManipulator';
import {
  blue,
  green,
  grey,
  indigo,
  orange,
  pink,
  purple,
  red,
  yellow,
} from '../palette';

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
      neutral: neutralColors,
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
    background: '#F0F2F4',
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
  },

  accessGraph: {
    dotsColor: 'rgba(0, 0, 0, 0.2)',
    edges: {
      dynamicMemberOf: {
        color: purple[700],
        stroke: purple[500],
      },
      memberOf: {
        color: 'rgba(0, 0, 0, 0.7)',
        stroke: '#c6c7c9',
      },
      reverse: {
        color: 'rgba(0, 0, 0, 0.7)',
        stroke: '#c6c7c9',
      },
      allowed: {
        color: green[700],
        stroke: green[300],
      },
      disallowed: {
        color: red[700],
        stroke: red[300],
      },
      restricted: {
        color: yellow[700],
        stroke: yellow[900],
      },
      default: {
        color: 'rgba(0, 0, 0, 0.7)',
        stroke: '#c6c7c9',
      },
      requestedBy: {
        color: 'rgba(0, 0, 0, 0.7)',
        stroke: '#c6c7c9',
      },
      requestedAction: {
        color: 'rgba(0, 0, 0, 0.7)',
        stroke: '#c6c7c9',
      },
      requestedResource: {
        color: 'rgba(0, 0, 0, 0.7)',
        stroke: '#c6c7c9',
      },
    },
    nodes: {
      user: {
        background: lighten(purple[300], 0.9),
        borderColor: purple[300],
        typeColor: purple[300],
        iconBackground: purple[300],
        handleColor: purple[704],
        highlightColor: purple[300],
        label: {
          background: purple[200],
          color: purple[700],
        },
      },
      userGroup: {
        background: lighten(orange[300], 0.9),
        borderColor: orange[300],
        typeColor: orange[300],
        iconBackground: orange[300],
        handleColor: orange[700],
        highlightColor: orange[300],
        label: {
          background: orange[200],
          color: orange[700],
        },
      },
      temporaryUserGroup: {
        background: lighten(orange[200], 0.9),
        borderColor: orange[200],
        typeColor: orange[200],
        iconBackground: orange[200],
        handleColor: orange[300],
        highlightColor: orange[200],
        label: {
          background: orange[200],
          color: orange[300],
        },
      },
      resource: {
        background: lighten(blue[300], 0.9),
        borderColor: blue[300],
        typeColor: blue[300],
        iconBackground: blue[300],
        handleColor: blue[400],
        highlightColor: blue[300],
        label: {
          background: blue[200],
          color: blue[700],
        },
      },
      resourceGroup: {
        background: lighten(pink[300], 0.9),
        borderColor: pink[300],
        typeColor: pink[300],
        iconBackground: pink[300],
        handleColor: pink[400],
        highlightColor: pink[300],
        label: {
          background: pink[200],
          color: pink[700],
        },
      },
      temporaryResourceGroup: {
        background: lighten(pink[200], 0.9),
        borderColor: pink[200],
        typeColor: pink[200],
        iconBackground: pink[200],
        handleColor: pink[300],
        highlightColor: pink[200],
        label: {
          background: pink[200],
          color: pink[300],
        },
      },
      allowedAction: {
        background: lighten(green[300], 0.9),
        borderColor: green[300],
        typeColor: green[300],
        iconBackground: green[300],
        handleColor: green[400],
        highlightColor: green[300],
        label: {
          background: green[200],
          color: green[700],
        },
      },
      temporaryAllowedAction: {
        background: lighten(green[200], 0.9),
        borderColor: green[200],
        typeColor: green[200],
        iconBackground: green[200],
        handleColor: green[300],
        highlightColor: green[200],
        label: {
          background: green[200],
          color: green[300],
        },
      },
      disallowedAction: {
        background: lighten(red[300], 0.9),
        borderColor: red[300],
        typeColor: red[300],
        iconBackground: red[300],
        handleColor: purple[400],
        highlightColor: red[300],
        label: {
          background: red[200],
          color: red[700],
        },
      },
      allowedRequest: {
        background: lighten(indigo[300], 0.9),
        borderColor: indigo[300],
        typeColor: indigo[300],
        iconBackground: indigo[300],
        handleColor: indigo[400],
        highlightColor: indigo[300],
        label: {
          background: indigo[200],
          color: indigo[700],
        },
      },
      disallowedRequest: {
        background: lighten(purple[300], 0.9),
        borderColor: purple[300],
        typeColor: purple[300],
        iconBackground: purple[300],
        handleColor: purple[400],
        highlightColor: purple[300],
        label: {
          background: purple[200],
          color: purple[700],
        },
      },
      allowedReview: {
        background: lighten(indigo[300], 0.9),
        borderColor: indigo[300],
        typeColor: indigo[300],
        iconBackground: indigo[300],
        handleColor: indigo[400],
        highlightColor: indigo[300],
        label: {
          background: indigo[200],
          color: indigo[700],
        },
      },
      disallowedReview: {
        background: lighten(purple[300], 0.9),
        borderColor: purple[300],
        typeColor: purple[300],
        iconBackground: purple[300],
        handleColor: purple[400],
        highlightColor: purple[300],
        label: {
          background: purple[200],
          color: purple[700],
        },
      },
      accessRequest: {
        background: lighten(grey[300], 0.9),
        borderColor: grey[300],
        typeColor: grey[700],
        iconBackground: grey[700],
        handleColor: grey[400],
        highlightColor: grey[300],
        label: {
          background: grey[200],
          color: grey[700],
        },
      },
    },
  },

  editor: {
    abbey: dataVisualisationColors.primary.abbey,
    purple: dataVisualisationColors.primary.purple,
    cyan: dataVisualisationColors.primary.cyan,
    picton: dataVisualisationColors.primary.picton,
    sunflower: dataVisualisationColors.primary.sunflower,
    caribbean: dataVisualisationColors.primary.caribbean,
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
