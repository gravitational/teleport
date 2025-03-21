import { defineSlotRecipe } from '@chakra-ui/react';

export const inputSlotRecipe = defineSlotRecipe({
  className: 'teleport-slot-input',
  slots: ['container', 'field', 'icon'],
  base: {
    container: {
      pos: 'relative',
      w: '100%',
    },
    field: {
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
    icon: {
      color: 'interactive.solid.danger.default',
    },
  },
  variants: {
    size: {
      sm: {
        field: {
          textStyle: 'sm',
          px: '2.5',
          '--input-height': 'sizes.9',
        },
        icon: {
          fontSize: '16px',
        },
      },
      md: {
        field: {
          textStyle: 'md',
          px: '4',
          '--input-height': 'sizes.11',
        },
        icon: {
          fontSize: '18px',
        },
      },
      lg: {
        field: {
          textStyle: 'md',
          px: '4',
          '--input-height': 'sizes.11',
        },
        icon: {
          fontSize: '20px',
        },
      },
    },
  },
  defaultVariants: {
    size: 'md',
  },
});
