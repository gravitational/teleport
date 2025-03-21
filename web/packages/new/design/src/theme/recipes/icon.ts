import { defineRecipe } from '@chakra-ui/react';

export const iconRecipe = defineRecipe({
  className: 'teleport-icon',
  base: {
    display: 'inline-block',
    lineHeight: '1em',
    flexShrink: '0',
    color: 'currentcolor',
    verticalAlign: 'middle',
  },
  variants: {
    size: {
      text: {
        width: '1em',
        height: '1em',
      },
      xs: {
        boxSize: '3',
      },
      sm: {
        boxSize: '4',
      },
      md: {
        boxSize: '5',
      },
      lg: {
        boxSize: '6',
      },
      xl: {
        boxSize: '7',
      },
      '2xl': {
        boxSize: '8',
      },
    },
  },
  defaultVariants: {
    size: 'text',
  },
});
