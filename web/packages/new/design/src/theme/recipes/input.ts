import { defineRecipe } from '@chakra-ui/react';

export const inputRecipe = defineRecipe({
  className: 'teleport-input',
  base: {
    bg: 'transparent',
    borderWidth: '1px',
    borderColor: 'interactive.tonal.neutral.2',
    focusVisibleRing: 'inside',
    focusRingColor: 'var(--focus-color)',
    width: '100%',
    minWidth: '0',
    outline: '0',
    position: 'relative',
    appearance: 'none',
    textAlign: 'start',
    borderRadius: 'l2',
    _disabled: {
      layerStyle: 'disabled',
    },
    height: 'var(--input-height)',
    minW: 'var(--input-height)',
    '--focus-color': 'colors.colorPalette.focusRing',
    '--error-color': 'colors.border.error',
    _invalid: {
      focusRingColor: 'var(--error-color)',
      borderColor: 'var(--error-color)',
    },
  },
  variants: {
    size: {
      sm: {
        textStyle: 'sm',
        px: '2.5',
        '--input-height': 'sizes.9',
      },
      md: {
        textStyle: 'md',
        px: '3',
        '--input-height': 'sizes.10',
      },
      lg: {
        textStyle: 'md',
        px: '4',
        '--input-height': 'sizes.11',
      },
    },
  },
  defaultVariants: {
    size: 'md',
  },
});
