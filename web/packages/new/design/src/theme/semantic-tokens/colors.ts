import { defineSemanticTokens } from '@chakra-ui/react';

export const colors = defineSemanticTokens.colors({
  levels: {
    deep: {
      value: {
        _light: '#E6E9EA',
        _dark: '#000000',
      },
    },
    sunken: {
      value: {
        _light: '#F1F2F4',
        _dark: '#0C143D',
      },
    },
    surface: {
      value: {
        _light: '#FBFBFC',
        _dark: '#222C59',
      },
    },
    elevated: {
      value: {
        _light: '#FFFFFF',
        _dark: '#344179',
      },
    },
    popout: {
      value: {
        _light: '#FFFFFF',
        _dark: '#4A5688',
      },
    },
  },
  interactive: {
    solid: {
      primary: {
        default: {
          value: {
            _light: '#512FC9',
            _dark: '#9F85FF',
          },
        },
        hover: {
          value: {
            _light: '#4126A1',
            _dark: '#B29DFF',
          },
        },
        active: {
          value: {
            _light: '#311C79',
            _dark: '#C5B6FF',
          },
        },
      },
      success: {
        default: {
          value: {
            _light: '#007D6B',
            _dark: '#00BFA6',
          },
        },
        hover: {
          value: {
            _light: '#006456',
            _dark: '#33CCB8',
          },
        },
        active: {
          value: {
            _light: '#004B40',
            _dark: '#66D9CA',
          },
        },
      },
      accent: {
        default: {
          value: {
            _light: '#0073BA',
            _dark: '#009EFF',
          },
        },
        hover: {
          value: {
            _light: '#005C95',
            _dark: '#33B1FF',
          },
        },
        active: {
          value: {
            _light: '#004570',
            _dark: '#66C5FF',
          },
        },
      },
      danger: {
        default: {
          value: {
            _light: '#CC372D',
            _dark: '#FF6257',
          },
        },
        hover: {
          value: {
            _light: '#A32C24',
            _dark: '#FF8179',
          },
        },
        active: {
          value: {
            _light: '#7A211B',
            _dark: '#FFA19A',
          },
        },
      },
      alert: {
        default: {
          value: {
            _light: '#FFAB00',
            _dark: '#FFAB00',
          },
        },
        hover: {
          value: {
            _light: '#CC8900',
            _dark: '#FFBC33',
          },
        },
        active: {
          value: {
            _light: '#996700',
            _dark: '#FFCD66',
          },
        },
      },
    },
    tonal: {
      primary: {
        '0': {
          value: {
            _light: 'rgba(81,47,201, 0.1)',
            _dark: 'rgba(159,133,255, 0.1)',
          },
        },
        '1': {
          value: {
            _light: 'rgba(81,47,201, 0.18)',
            _dark: 'rgba(159,133,255, 0.18)',
          },
        },
        '2': {
          value: {
            _light: 'rgba(81,47,201, 0.25)',
            _dark: 'rgba(159,133,255, 0.25)',
          },
        },
      },
      success: {
        '0': {
          value: {
            _light: 'rgba(0, 125, 107, 0.1)',
            _dark: 'rgba(0, 191, 166, 0.1)',
          },
        },
        '1': {
          value: {
            _light: 'rgba(0, 125, 107, 0.18)',
            _dark: 'rgba(0, 191, 166, 0.18)',
          },
        },
        '2': {
          value: {
            _light: 'rgba(0, 125, 107, 0.25)',
            _dark: 'rgba(0, 191, 166, 0.25)',
          },
        },
      },
      danger: {
        '0': {
          value: {
            _light: 'rgba(204, 55, 45, 0.1)',
            _dark: 'rgba(255, 98, 87, 0.1)',
          },
        },
        '1': {
          value: {
            _light: 'rgba(204, 55, 45, 0.18)',
            _dark: 'rgba(255, 98, 87, 0.18)',
          },
        },
        '2': {
          value: {
            _light: 'rgba(204, 55, 45, 0.25)',
            _dark: 'rgba(255, 98, 87, 0.25)',
          },
        },
      },
      alert: {
        '0': {
          value: {
            _light: 'rgba(255, 171, 0, 0.1)',
            _dark: 'rgba(255, 171, 0, 0.1)',
          },
        },
        '1': {
          value: {
            _light: 'rgba(255, 171, 0, 0.18)',
            _dark: 'rgba(255, 171, 0, 0.18)',
          },
        },
        '2': {
          value: {
            _light: 'rgba(255, 171, 0, 0.25)',
            _dark: 'rgba(255, 171, 0, 0.25)',
          },
        },
      },
      informational: {
        '0': {
          value: {
            _light: 'rgba(0, 115, 186, 0.1)',
            _dark: 'rgba(0, 158, 255, 0.1)',
          },
        },
        '1': {
          value: {
            _light: 'rgba(0, 115, 186, 0.18)',
            _dark: 'rgba(0, 158, 255, 0.18)',
          },
        },
        '2': {
          value: {
            _light: 'rgba(0, 115, 186, 0.25)',
            _dark: 'rgba(0, 158, 255, 0.25)',
          },
        },
      },
      neutral: {
        '0': {
          value: {
            _light: 'rgba(0,0,0,0.06)',
            _dark: 'rgba(255,255,255,0.07)',
          },
        },
        '1': {
          value: {
            _light: 'rgba(0,0,0,0.13)',
            _dark: 'rgba(255,255,255,0.13)',
          },
        },
        '2': {
          value: {
            _light: 'rgba(0,0,0,0.18)',
            _dark: 'rgba(255,255,255,0.18)',
          },
        },
      },
    },
  },
  text: {
    main: {
      value: {
        _light: '#000000',
        _dark: '#FFFFFF',
      },
    },
    slightlyMuted: {
      value: {
        _light: 'rgba(0,0,0,0.72)',
        _dark: 'rgba(255, 255, 255, 0.72)',
      },
    },
    muted: {
      value: {
        _light: 'rgba(0,0,0,0.54)',
        _dark: 'rgba(255, 255, 255, 0.54)',
      },
    },
    disabled: {
      value: {
        _light: 'rgba(0,0,0,0.36)',
        _dark: 'rgba(255, 255, 255, 0.36)',
      },
    },
    primaryInverse: {
      value: {
        _light: '#FFFFFF',
        _dark: '#000000',
      },
    },
  },
  buttons: {
    text: {
      value: {
        _light: '#000000',
        _dark: '#FFFFFF',
      },
    },
    textDisabled: {
      value: {
        _light: 'rgba(0,0,0,0.3)',
        _dark: 'rgba(255, 255, 255, 0.3)',
      },
    },
    bgDisabled: {
      value: {
        _light: 'rgba(0,0,0,0.12)',
        _dark: 'rgba(255, 255, 255, 0.12)',
      },
    },
    border: {
      default: {
        value: {
          _light: 'rgba(255,255,255,0)',
          _dark: 'rgba(255,255,255,0)',
        },
      },
      hover: {
        value: {
          _light: 'rgba(0,0,0,0.07)',
          _dark: 'rgba(255, 255, 255, 0.07)',
        },
      },
      active: {
        value: {
          _light: 'rgba(0,0,0,0.13)',
          _dark: 'rgba(255, 255, 255, 0.13)',
        },
      },
      border: {
        value: {
          _light: 'rgba(0,0,0,0.36)',
          _dark: 'rgba(255, 255, 255, 0.36)',
        },
      },
    },
    trashButton: {
      default: {
        value: {
          _light: 'rgba(0,0,0,0.07)',
          _dark: 'rgba(255, 255, 255, 0.07)',
        },
      },
      hover: {
        value: {
          _light: 'rgba(0,0,0,0.13)',
          _dark: 'rgba(255, 255, 255, 0.13)',
        },
      },
    },
  },
  tooltip: {
    background: {
      value: {
        _light: 'rgba(0, 0, 0, 0.80)',
        _dark: 'rgba(255, 255, 255, 0.8)',
      },
    },
    inverseBackground: {
      value: {
        _light: 'rgba(255, 255, 255, 0.5)',
        _dark: 'rgba(0, 0, 0, 0.5)',
      },
    },
  },
  terminal: {
    foreground: {
      value: {
        _light: '#000',
        _dark: '#FFF',
      },
    },
    background: {
      value: {
        _light: '#F1F2F4',
        _dark: '#0C143D',
      },
    },
    selectionBackground: {
      value: {
        _light: 'rgba(0, 0, 0, 0.18)',
        _dark: 'rgba(255, 255, 255, 0.18)',
      },
    },
    cursor: {
      value: {
        _light: '#000',
        _dark: '#FFF',
      },
    },
    cursorAccent: {
      value: {
        _light: '#F1F2F4',
        _dark: '#0C143D',
      },
    },
    red: {
      value: {
        _light: '#9D0A00',
        _dark: '#FF6257',
      },
    },
    green: {
      value: {
        _light: '#005742',
        _dark: '#00BFA6',
      },
    },
    yellow: {
      value: {
        _light: '#704B00',
        _dark: '#FFAB00',
      },
    },
    blue: {
      value: {
        _light: '#004B89',
        _dark: '#009EFF',
      },
    },
    magenta: {
      value: {
        _light: '#3D1BB2',
        _dark: '#9F85FF',
      },
    },
    cyan: {
      value: {
        _light: '#015C6E',
        _dark: '#00D3F0',
      },
    },
    brightWhite: {
      value: {
        _light: 'rgb(108, 108, 109)',
        _dark: 'rgb(228, 229, 233)',
      },
    },
    white: {
      value: {
        _light: 'rgb(77, 77, 78)',
        _dark: 'rgb(201, 203, 212)',
      },
    },
    brightBlack: {
      value: {
        _light: 'rgb(48, 48, 48)',
        _dark: 'rgb(160, 163, 179)',
      },
    },
    black: {
      value: {
        _light: '#000',
        _dark: '#000',
      },
    },
    brightRed: {
      value: {
        _light: '#BF372E',
        _dark: '#FF948D',
      },
    },
    brightGreen: {
      value: {
        _light: '#007562',
        _dark: '#2EFFD5',
      },
    },
    brightYellow: {
      value: {
        _light: '#8F5F00',
        _dark: '#FFD98C',
      },
    },
    brightBlue: {
      value: {
        _light: '#006BB8',
        _dark: '#7BCDFF',
      },
    },
    brightMagenta: {
      value: {
        _light: '#5531D4',
        _dark: '#B9A6FF',
      },
    },
    brightCyan: {
      value: {
        _light: '#007282',
        _dark: '#74EEFF',
      },
    },
    searchMatch: {
      value: {
        _light: '#FFD98C',
        _dark: '#FFD98C',
      },
    },
    activeSearchMatch: {
      value: {
        _light: '#FFAB00',
        _dark: '#FFAB00',
      },
    },
  },
  editor: {
    abbey: {
      value: {
        _light: '#BF372E',
        _dark: '#FF948D',
      },
    },
    purple: {
      value: {
        _light: '#5531D4',
        _dark: '#B9A6FF',
      },
    },
    cyan: {
      value: {
        _light: '#007282',
        _dark: '#74EEFF',
      },
    },
    picton: {
      value: {
        _light: '#006BB8',
        _dark: '#7BCDFF',
      },
    },
    sunflower: {
      value: {
        _light: '#8F5F00',
        _dark: '#FFD98C',
      },
    },
    caribbean: {
      value: {
        _light: '#007562',
        _dark: '#2EFFD5',
      },
    },
  },
  progressBarColor: {
    value: {
      _light: '#007D6B',
      _dark: '#00BFA5',
    },
  },
  link: {
    value: {
      _light: '#0073BA',
      _dark: '#009EFF',
    },
  },
  highlightedNavigationItem: {
    value: {
      _light: '#90caf9',
      _dark: 'rgba(255, 255, 255, 0.3)',
    },
  },
  notice: {
    background: {
      value: {
        _light: '#e3f2fd',
        _dark: '#344179',
      },
    },
  },
  action: {
    active: {
      value: {
        _light: '#FFFFFF',
        _dark: '#FFFFFF',
      },
    },
    hover: {
      value: {
        _light: 'rgba(255, 255, 255, 0.1)',
        _dark: 'rgba(255, 255, 255, 0.1)',
      },
    },
    hoverOpacity: {
      value: 0.1,
    },
    selected: {
      value: {
        _light: 'rgba(255, 255, 255, 0.2)',
        _dark: 'rgba(255, 255, 255, 0.2)',
      },
    },
    disabled: {
      value: {
        _light: 'rgba(255, 255, 255, 0.3)',
        _dark: 'rgba(255, 255, 255, 0.3)',
      },
    },
    disabledBackground: {
      value: {
        _light: 'rgba(255, 255, 255, 0.12)',
        _dark: 'rgba(255, 255, 255, 0.12)',
      },
    },
  },
  bg: {
    DEFAULT: {
      value: {
        _light: '{colors.white}',
        _dark: '{colors.black}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.gray.50}',
        _dark: '{colors.gray.950}',
      },
    },
    muted: {
      value: {
        _light: '{colors.gray.100}',
        _dark: '{colors.gray.900}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.gray.200}',
        _dark: '{colors.gray.800}',
      },
    },
    inverted: {
      value: {
        _light: '{colors.black}',
        _dark: '{colors.white}',
      },
    },
    panel: {
      value: {
        _light: '{colors.white}',
        _dark: '{colors.gray.950}',
      },
    },
    error: {
      value: {
        _light: '{colors.red.50}',
        _dark: '{colors.red.950}',
      },
    },
    warning: {
      value: {
        _light: '{colors.orange.50}',
        _dark: '{colors.orange.950}',
      },
    },
    success: {
      value: {
        _light: '{colors.green.50}',
        _dark: '{colors.green.950}',
      },
    },
    info: {
      value: {
        _light: '{colors.blue.50}',
        _dark: '{colors.blue.950}',
      },
    },
  },
  fg: {
    DEFAULT: {
      value: {
        _light: '{colors.black}',
        _dark: '{colors.gray.50}',
      },
    },
    muted: {
      value: {
        _light: '{colors.gray.600}',
        _dark: '{colors.gray.400}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.gray.400}',
        _dark: '{colors.gray.500}',
      },
    },
    inverted: {
      value: {
        _light: '{colors.gray.50}',
        _dark: '{colors.black}',
      },
    },
    error: {
      value: {
        _light: '{colors.red.500}',
        _dark: '{colors.red.400}',
      },
    },
    warning: {
      value: {
        _light: '{colors.orange.600}',
        _dark: '{colors.orange.300}',
      },
    },
    success: {
      value: {
        _light: '{colors.green.600}',
        _dark: '{colors.green.300}',
      },
    },
    info: {
      value: {
        _light: '{colors.blue.600}',
        _dark: '{colors.blue.300}',
      },
    },
  },
  border: {
    DEFAULT: {
      value: {
        _light: '{colors.gray.200}',
        _dark: '{colors.gray.800}',
      },
    },
    muted: {
      value: {
        _light: '{colors.gray.100}',
        _dark: '{colors.gray.900}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.gray.50}',
        _dark: '{colors.gray.950}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.gray.300}',
        _dark: '{colors.gray.700}',
      },
    },
    inverted: {
      value: {
        _light: '{colors.gray.800}',
        _dark: '{colors.gray.200}',
      },
    },
    error: {
      value: {
        _light: '{colors.red.500}',
        _dark: '{colors.red.400}',
      },
    },
    warning: {
      value: {
        _light: '{colors.orange.500}',
        _dark: '{colors.orange.400}',
      },
    },
    success: {
      value: {
        _light: '{colors.green.500}',
        _dark: '{colors.green.400}',
      },
    },
    info: {
      value: {
        _light: '{colors.blue.500}',
        _dark: '{colors.blue.400}',
      },
    },
  },
  gray: {
    contrast: {
      value: {
        _light: '{colors.white}',
        _dark: '{colors.black}',
      },
    },
    fg: {
      value: {
        _light: '{colors.gray.800}',
        _dark: '{colors.gray.200}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.gray.100}',
        _dark: '{colors.gray.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.gray.200}',
        _dark: '{colors.gray.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.gray.300}',
        _dark: '{colors.gray.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.gray.900}',
        _dark: '{colors.white}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.gray.800}',
        _dark: '{colors.gray.200}',
      },
    },
  },
  red: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.red.700}',
        _dark: '{colors.red.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.red.100}',
        _dark: '{colors.red.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.red.200}',
        _dark: '{colors.red.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.red.300}',
        _dark: '{colors.red.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.red.600}',
        _dark: '{colors.red.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.red.600}',
        _dark: '{colors.red.600}',
      },
    },
  },
  orange: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'black',
      },
    },
    fg: {
      value: {
        _light: '{colors.orange.700}',
        _dark: '{colors.orange.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.orange.100}',
        _dark: '{colors.orange.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.orange.200}',
        _dark: '{colors.orange.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.orange.300}',
        _dark: '{colors.orange.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.orange.600}',
        _dark: '{colors.orange.500}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.orange.600}',
        _dark: '{colors.orange.500}',
      },
    },
  },
  green: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.green.700}',
        _dark: '{colors.green.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.green.100}',
        _dark: '{colors.green.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.green.200}',
        _dark: '{colors.green.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.green.300}',
        _dark: '{colors.green.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.green.600}',
        _dark: '{colors.green.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.green.600}',
        _dark: '{colors.green.600}',
      },
    },
  },
  blue: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.blue.700}',
        _dark: '{colors.blue.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.blue.100}',
        _dark: '{colors.blue.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.blue.200}',
        _dark: '{colors.blue.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.blue.300}',
        _dark: '{colors.blue.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.blue.600}',
        _dark: '{colors.blue.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.blue.600}',
        _dark: '{colors.blue.600}',
      },
    },
  },
  yellow: {
    contrast: {
      value: {
        _light: 'black',
        _dark: 'black',
      },
    },
    fg: {
      value: {
        _light: '{colors.yellow.800}',
        _dark: '{colors.yellow.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.yellow.100}',
        _dark: '{colors.yellow.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.yellow.200}',
        _dark: '{colors.yellow.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.yellow.300}',
        _dark: '{colors.yellow.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.yellow.300}',
        _dark: '{colors.yellow.300}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.yellow.300}',
        _dark: '{colors.yellow.300}',
      },
    },
  },
  teal: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.teal.700}',
        _dark: '{colors.teal.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.teal.100}',
        _dark: '{colors.teal.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.teal.200}',
        _dark: '{colors.teal.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.teal.300}',
        _dark: '{colors.teal.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.teal.600}',
        _dark: '{colors.teal.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.teal.600}',
        _dark: '{colors.teal.600}',
      },
    },
  },
  purple: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.purple.700}',
        _dark: '{colors.purple.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.purple.100}',
        _dark: '{colors.purple.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.purple.200}',
        _dark: '{colors.purple.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.purple.300}',
        _dark: '{colors.purple.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.purple.600}',
        _dark: '{colors.purple.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.purple.600}',
        _dark: '{colors.purple.600}',
      },
    },
  },
  pink: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.pink.700}',
        _dark: '{colors.pink.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.pink.100}',
        _dark: '{colors.pink.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.pink.200}',
        _dark: '{colors.pink.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.pink.300}',
        _dark: '{colors.pink.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.pink.600}',
        _dark: '{colors.pink.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.pink.600}',
        _dark: '{colors.pink.600}',
      },
    },
  },
  cyan: {
    contrast: {
      value: {
        _light: 'white',
        _dark: 'white',
      },
    },
    fg: {
      value: {
        _light: '{colors.cyan.700}',
        _dark: '{colors.cyan.300}',
      },
    },
    subtle: {
      value: {
        _light: '{colors.cyan.100}',
        _dark: '{colors.cyan.900}',
      },
    },
    muted: {
      value: {
        _light: '{colors.cyan.200}',
        _dark: '{colors.cyan.800}',
      },
    },
    emphasized: {
      value: {
        _light: '{colors.cyan.300}',
        _dark: '{colors.cyan.700}',
      },
    },
    solid: {
      value: {
        _light: '{colors.cyan.600}',
        _dark: '{colors.cyan.600}',
      },
    },
    focusRing: {
      value: {
        _light: '{colors.cyan.600}',
        _dark: '{colors.cyan.600}',
      },
    },
  },
});
