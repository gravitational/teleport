import { defineSemanticTokens } from '@chakra-ui/react';

export const shadows = defineSemanticTokens.shadows({
  xs: {
    value:
      '0px 2px 1px -1px rgba(0, 0, 0, 0.2), 0px 1px 1px rgba(0, 0, 0, 0.14), 0px 1px 3px rgba(0, 0, 0, 0.12)',
  },
  sm: {
    value:
      '0px 5px 5px -3px rgba(0, 0, 0, 0.2), 0px 8px 10px 1px rgba(0, 0, 0, 0.14), 0px 3px 14px 2px rgba(0, 0, 0, 0.12)',
  },
  md: {
    value:
      '0px 3px 5px -1px rgba(0, 0, 0, 0.2), 0px 6px 10px rgba(0, 0, 0, 0.14), 0px 1px 18px rgba(0, 0, 0, 0.12)',
  },
  lg: {
    value:
      '0px 1px 10px 0px rgba(0, 0, 0, 0.12), 0px 4px 5px 0px rgba(0, 0, 0, 0.14), 0px 2px 4px -1px rgba(0, 0, 0, 0.20)',
  },
  // xs: {
  //   value: {
  //     _light:
  //       '0px 1px 2px {colors.gray.900/10}, 0px 0px 1px {colors.gray.900/20}',
  //     _dark: '0px 1px 1px {black/64}, 0px 0px 1px inset {colors.gray.300/20}',
  //   },
  // },
  // sm: {
  //   value: {
  //     _light:
  //       '0px 2px 4px {colors.gray.900/10}, 0px 0px 1px {colors.gray.900/30}',
  //     _dark: '0px 2px 4px {black/64}, 0px 0px 1px inset {colors.gray.300/30}',
  //   },
  // },
  // md: {
  //   value: {
  //     _light:
  //       '0px 4px 8px {colors.gray.900/10}, 0px 0px 1px {colors.gray.900/30}',
  //     _dark: '0px 4px 8px {black/64}, 0px 0px 1px inset {colors.gray.300/30}',
  //   },
  // },
  // lg: {
  //   value: {
  //     _light:
  //       '0px 8px 16px {colors.gray.900/10}, 0px 0px 1px {colors.gray.900/30}',
  //     _dark: '0px 8px 16px {black/64}, 0px 0px 1px inset {colors.gray.300/30}',
  //   },
  // },
  xl: {
    value: {
      _light:
        '0px 16px 24px {colors.gray.900/10}, 0px 0px 1px {colors.gray.900/30}',
      _dark: '0px 16px 24px {black/64}, 0px 0px 1px inset {colors.gray.300/30}',
    },
  },
  '2xl': {
    value: {
      _light:
        '0px 24px 40px {colors.gray.900/16}, 0px 0px 1px {colors.gray.900/30}',
      _dark: '0px 24px 40px {black/64}, 0px 0px 1px inset {colors.gray.300/30}',
    },
  },
  inner: {
    value: {
      _light: 'inset 0 2px 4px 0 {black/5}',
      _dark: 'inset 0 2px 4px 0 black',
    },
  },
  inset: {
    value: {
      _light: 'inset 0 0 0 1px {black/5}',
      _dark: 'inset 0 0 0 1px {colors.gray.300/5}',
    },
  },
});
