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

import { darken, lighten } from '../utils/colorManipulator';
import { blue, green, orange, pink, purple, red, yellow } from '../palette';

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

const colors: ThemeColors = {
  ...sharedColors,

  levels,

  spotBackground: ['rgba(0,0,0,0.06)', 'rgba(0,0,0,0.13)', 'rgba(0,0,0,0.18)'],

  brand: '#512FC9',

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
        color: blue[700],
        stroke: blue[300],
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
  success: '#007D6B',

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
