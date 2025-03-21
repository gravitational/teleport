import { defineRecipe } from '@chakra-ui/react';

export const buttonRecipe = defineRecipe({
  className: 'teleport-button',
  base: {
    border: '1.5px solid transparent',
    display: 'inline-flex',
    appearance: 'none',
    alignItems: 'center',
    justifyContent: 'center',
    userSelect: 'none',
    position: 'relative',
    borderRadius: 'l2',
    whiteSpace: 'nowrap',
    verticalAlign: 'middle',
    borderColor: 'transparent',
    cursor: 'button',
    flexShrink: '0',
    lineHeight: '1.2',
    isolation: 'isolate',
    fontWeight: 'semibold',
    transitionProperty: 'common',
    transitionDuration: 'moderate',
    _focusVisible: {
      outline: '2px solid var(--bg-currentcolor)',
    },
    _disabled: {
      layerStyle: 'disabled',
    },
    _icon: {
      flexShrink: '0',
    },
  },
  variants: {
    size: {
      sm: {
        minH: 7,
        px: 'calc(var(--teleport-spacing-3) - 1.5px)',
        textStyle: 'sm',
        gap: '2',
        _icon: {
          width: '4',
          height: '4',
        },
      },
      md: {
        minH: 9,
        minW: '10',
        textStyle: 'sm',
        px: 'calc(var(--teleport-spacing-9) - 1.5px)',
        gap: '2',
        _icon: {
          width: '5',
          height: '5',
        },
      },
      lg: {
        minH: 11,
        textStyle: 'lg',
        px: 'calc(var(--teleport-spacing-9) - 1.5px)',
        gap: '3',
        _icon: {
          width: '5',
          height: '5',
        },
      },
      xl: {
        minH: '3.1rem',
        textStyle: 'xl',
        px: 'calc(var(--teleport-spacing-9) - 1.5px)',
        gap: '2.5',
        _icon: {
          width: '5',
          height: '5',
        },
      },
    },
    intent: {
      primary: {},
      neutral: {},
      danger: {},
      success: {},
    },
    variant: {
      filled: {
        color: 'text.primaryInverse',
      },
      minimal: {},
      border: {},
    },
    block: {
      true: {
        w: '100%',
      },
    },
    compact: {
      true: {
        px: 'calc(var(--teleport-spacing-1) - 1.5px)',
      },
    },
    inputAlignment: {
      true: {
        px: 'calc(var(--teleport-spacing-4) - 1.5px)',
      },
    },
  },
  compoundVariants: [
    // filled
    {
      intent: 'primary',
      variant: 'filled',
      css: {
        bg: 'interactive.solid.primary.default',
        _hover: {
          bg: 'interactive.solid.primary.hover',
        },
        _active: {
          bg: 'interactive.solid.primary.active',
        },
        _focusVisible: {
          bg: 'interactive.solid.primary.default',
          borderColor: 'text.primaryInverse',
        },
      },
    },
    {
      intent: 'danger',
      variant: 'filled',
      css: {
        bg: 'interactive.solid.danger.default',
        _hover: {
          bg: 'interactive.solid.danger.hover',
        },
        _active: {
          bg: 'interactive.solid.danger.active',
        },
        _focusVisible: {
          bg: 'interactive.solid.danger.default',
          borderColor: 'text.primaryInverse',
        },
      },
    },
    {
      intent: 'success',
      variant: 'filled',
      css: {
        bg: 'interactive.solid.success.default',
        _hover: {
          bg: 'interactive.solid.success.hover',
        },
        _active: {
          bg: 'interactive.solid.success.active',
        },
        _focusVisible: {
          bg: 'interactive.solid.success.default',
          borderColor: 'text.primaryInverse',
        },
      },
    },
    {
      intent: 'neutral',
      variant: 'filled',
      css: {
        color: 'text.slightlyMuted',
        bg: 'interactive.tonal.neutral.0',
        _hover: {
          color: 'text.main',
          bg: 'interactive.tonal.neutral.1',
        },
        _active: {
          color: 'text.main',
          bg: 'interactive.tonal.neutral.2',
        },
        _focusVisible: {
          color: 'text.slightlyMuted',
          bg: 'interactive.tonal.neutral.0',
          borderColor: 'text.primaryInverse',
        },
      },
    },

    // minimal
    {
      intent: 'primary',
      variant: 'minimal',
      css: {
        bg: 'transparent',
        color: 'interactive.solid.primary.default',
        _hover: {
          bg: 'interactive.tonal.primary.0',
          color: 'interactive.solid.primary.hover',
        },
        _active: {
          bg: 'interactive.tonal.primary.1',
          color: 'interactive.solid.primary.active',
        },
        _focusVisible: {
          color: 'interactive.solid.primary.default',
          borderColor: 'interactive.solid.primary.default',
          outline: 0,
        },
      },
    },
    {
      intent: 'danger',
      variant: 'minimal',
      css: {
        bg: 'transparent',
        color: 'interactive.solid.danger.default',
        _hover: {
          bg: 'interactive.tonal.danger.0',
          color: 'interactive.solid.danger.hover',
        },
        _active: {
          bg: 'interactive.tonal.danger.1',
          color: 'interactive.solid.danger.active',
        },
        _focusVisible: {
          color: 'interactive.solid.danger.default',
          borderColor: 'interactive.solid.danger.default',
          outline: 0,
        },
      },
    },
    {
      intent: 'success',
      variant: 'minimal',
      css: {
        bg: 'transparent',
        color: 'interactive.solid.success.default',
        _hover: {
          bg: 'interactive.tonal.success.0',
          color: 'interactive.solid.success.hover',
        },
        _active: {
          bg: 'interactive.tonal.success.1',
          color: 'interactive.solid.success.active',
        },
        _focusVisible: {
          color: 'interactive.solid.success.default',
          borderColor: 'interactive.solid.success.default',
          outline: 0,
        },
      },
    },
    {
      intent: 'neutral',
      variant: 'minimal',
      css: {
        color: 'text.slightlyMuted',
        bg: 'transparent',
        _hover: {
          bg: 'interactive.tonal.neutral.0',
        },
        _active: {
          color: 'text.main',
          bg: 'interactive.tonal.neutral.1',
        },
        _focusVisible: {
          color: 'text.slightlyMuted',
          borderColor: 'text.slightlyMuted',
        },
      },
    },

    // border
    {
      intent: 'primary',
      variant: 'border',
      css: {
        bg: 'transparent',
        color: 'interactive.solid.primary.default',
        borderColor: 'interactive.solid.primary.default',
        _hover: {
          color: 'text.primaryInverse',
          bg: 'interactive.solid.primary.hover',
          borderColor: 'transparent',
        },
        _active: {
          color: 'text.primaryInverse',
          bg: 'interactive.solid.primary.active',
          borderColor: 'transparent',
        },
        _focusVisible: {
          color: 'text.primaryInverse',
          borderColor: 'text.primaryInverse',
          bg: 'interactive.solid.primary.default',
        },
      },
    },
    {
      intent: 'danger',
      variant: 'border',
      css: {
        bg: 'transparent',
        color: 'interactive.solid.danger.default',
        borderColor: 'interactive.solid.danger.default',
        _hover: {
          color: 'text.primaryInverse',
          bg: 'interactive.solid.danger.hover',
          borderColor: 'transparent',
        },
        _active: {
          color: 'text.primaryInverse',
          bg: 'interactive.solid.danger.active',
          borderColor: 'transparent',
        },
        _focusVisible: {
          color: 'text.primaryInverse',
          borderColor: 'text.primaryInverse',
          bg: 'interactive.solid.danger.default',
        },
      },
    },
    {
      intent: 'success',
      variant: 'border',
      css: {
        bg: 'transparent',
        color: 'interactive.solid.success.default',
        borderColor: 'interactive.solid.success.default',
        _hover: {
          color: 'text.primaryInverse',
          bg: 'interactive.solid.success.hover',
          borderColor: 'transparent',
        },
        _active: {
          color: 'text.primaryInverse',
          bg: 'interactive.solid.success.active',
          borderColor: 'transparent',
        },
        _focusVisible: {
          color: 'text.primaryInverse',
          borderColor: 'text.primaryInverse',
          bg: 'interactive.solid.success.default',
        },
      },
    },
    {
      intent: 'neutral',
      variant: 'border',
      css: {
        bg: 'transparent',
        color: 'text.slightlyMuted',
        borderColor: 'interactive.tonal.neutral.2',
        _hover: {
          color: 'text.main',
          bg: 'interactive.tonal.neutral.1',
          borderColor: 'transparent',
        },
        _active: {
          color: 'text.main',
          bg: 'interactive.tonal.neutral.2',
          borderColor: 'transparent',
        },
        _focusVisible: {
          color: 'text.slightlyMuted',
          borderColor: 'text.slightlyMuted',
          bg: 'interactive.tonal.neutral.0',
        },
      },
    },
  ],
  defaultVariants: {
    size: 'md',
    variant: 'filled',
    intent: 'primary',
  },
});
