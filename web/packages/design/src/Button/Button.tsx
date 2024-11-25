/*
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

import React from 'react';
import styled, { CSSObject } from 'styled-components';

import {
  alignSelf,
  AlignSelfProps,
  gap,
  GapProps,
  height,
  HeightProps,
  space,
  SpaceProps,
  width,
  WidthProps,
} from 'design/system';
import { Theme } from 'design/theme/themes/types';
import { shouldForwardProp as defaultValidatorFn } from 'design/ThemeProvider';

export type ButtonProps<E extends React.ElementType> =
  React.ComponentPropsWithoutRef<E> &
    SpaceProps &
    WidthProps &
    HeightProps &
    AlignSelfProps &
    GapProps & {
      /**
       * Specifies if an element's display is set to block or not. Set to true
       * to set display to block.
       */
      block?: boolean;

      disabled?: boolean;

      /** Fill specifies desired shape of the button. */
      fill?: ButtonFill;

      /** Specifies the button's purpose class and affects its color palette. */
      intent?: ButtonIntent;

      /**
       * If set to true, renders a button with horizontal padding equal to the
       * size of input margins, thus allowing alignment between input text and
       * button labels.
       */
      inputAlignment?: boolean;

      /**
       * If set to true, renders a button with the smallest horizontal paddings.
       */
      compact?: boolean;

      /**
       * Specifies the case transform of the button text. Default is no
       * transformation.
       */
      textTransform?: string;

      size?: ButtonSize;
      children?: React.ReactNode;
      setRef?: React.ForwardedRef<HTMLButtonElement>;

      /** If defined, changes the underlying component type. */
      as?: E;
    };

export type ButtonFill = 'filled' | 'minimal' | 'border';
export type ButtonIntent = 'neutral' | 'primary' | 'danger' | 'success';
export type ButtonSize = 'extra-large' | 'large' | 'medium' | 'small';

/**
 * A generic button component. You can use `fill` and `intent` to pick the right
 * button intent and appearance or use one of the `ButtonXxx` wrappers to render
 * one of the typical buttons.
 */
export const Button = <E extends React.ElementType = 'button'>({
  children,
  setRef = undefined,
  size = 'medium',
  intent = 'primary',
  fill = 'filled',
  ...otherProps
}: ButtonProps<E>) => {
  return (
    <StyledButton
      {...otherProps}
      ref={setRef}
      size={size}
      intent={intent}
      fill={fill}
    >
      {children}
    </StyledButton>
  );
};

Button.displayName = 'Button';

export type ThemedButtonProps<E extends React.ElementType> = ButtonProps<E> & {
  theme: Theme;
  fill: ButtonFill;
  intent: ButtonIntent;
  size: ButtonSize;
};

const themedStyles = <E extends React.ElementType>(
  props: ThemedButtonProps<E>
): CSSObject => {
  const { colors } = props.theme;

  const style = buttonStyle(props);

  let disabledStyle: CSSObject = {
    backgroundColor: colors.interactive.tonal.neutral[0],
    color: colors.buttons.textDisabled,
    borderColor: 'transparent',
    boxShadow: 'none',
    cursor: 'auto',
  };
  style['&:disabled'] = disabledStyle;

  // Using the pseudo class `:disabled` to style disabled state
  // doesn't work for non form elements (e.g. anchor). So
  // we target by attribute with square brackets. Only true
  // when we change the underlying type for this component (button)
  // using the `as` prop (eg: a, NavLink, Link).
  if (props.as && props.disabled) {
    disabledStyle.pointerEvents = 'none';
    style['&[disabled]'] = disabledStyle;
  }

  return {
    ...style,
    ...size(props),
    ...space(props),
    ...width(props),
    ...block(props),
    ...height(props),
    ...textTransform(props),
    ...alignSelf(props),
    // Since a Button has `display: inline-flex`, we want to be able to set gap within it in case we
    // need to use an icon.
    ...gap(props),
  };
};

const buttonStyle = <E extends React.ElementType>(
  props: ThemedButtonProps<E>
): CSSObject => {
  const { fill, intent } = props;
  const palette = buttonPalette(props);
  return {
    backgroundColor: palette.default.background,
    color: palette.default.text,
    borderColor: palette.default.border ?? 'transparent',
    ['&:focus-visible, .teleport-button__force-focus-visible &']: {
      backgroundColor: palette.focus.background,
      color: palette.focus.text,
      borderColor: palette.focus.border ?? 'transparent',
      borderRadius: intent === 'neutral' || fill === 'minimal' ? '4px' : '2px',
      outline:
        intent !== 'neutral' && fill !== 'minimal'
          ? `2px solid ${palette.focus.background}`
          : 'none',
    },
    '&:hover, .teleport-button__force-hover &': {
      backgroundColor: palette.hover.background,
      borderColor: palette.hover.border ?? 'transparent',
      color: palette.hover.text,
      boxShadow:
        intent === 'neutral' || fill === 'minimal'
          ? 'none'
          : `0px 3px 5px -1px rgba(0, 0, 0, 0.20),
            0px 5px 8px 0px rgba(0, 0, 0, 0.14),
            0px 1px 14px 0px rgba(0, 0, 0, 0.12)`,
    },
    '&:active, .teleport-button__force-active &': {
      backgroundColor: palette.active.background,
      borderColor: palette.active.border ?? 'transparent',
      color: palette.active.text,
    },
  };
};

type ButtonPaletteEntry = {
  text: string;
  background: string;
  border?: string;
};

type ButtonPalette = {
  default: ButtonPaletteEntry;
  hover: ButtonPaletteEntry;
  active: ButtonPaletteEntry;
  focus: ButtonPaletteEntry;
};

const buttonPalette = <E extends React.ElementType>({
  theme: { colors },
  intent,
  fill,
}: ThemedButtonProps<E>): ButtonPalette => {
  switch (fill) {
    case 'filled':
      if (intent === 'neutral') {
        return {
          default: {
            text: colors.text.slightlyMuted,
            background: colors.interactive.tonal.neutral[0],
          },
          hover: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[1],
          },
          active: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[2],
          },
          focus: {
            text: colors.text.slightlyMuted,
            border: colors.text.slightlyMuted,
            background: colors.interactive.tonal.neutral[0],
          },
        };
      } else {
        return {
          default: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].default,
          },
          hover: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].hover,
          },
          active: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].active,
          },
          focus: {
            text: colors.text.primaryInverse,
            border: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].default,
          },
        };
      }
    case 'minimal': {
      if (intent === 'neutral') {
        return {
          default: {
            text: colors.text.slightlyMuted,
            background: 'transparent',
          },
          hover: {
            text: colors.text.slightlyMuted,
            background: colors.interactive.tonal[intent][0],
          },
          active: {
            text: colors.text.main,
            background: colors.interactive.tonal[intent][1],
          },
          focus: {
            text: colors.text.slightlyMuted,
            border: colors.text.slightlyMuted,
            background: 'transparent',
          },
        };
      }
      return {
        default: {
          text: colors.interactive.solid[intent].default,
          background: 'transparent',
        },
        hover: {
          text: colors.interactive.solid[intent].hover,
          background: colors.interactive.tonal[intent][0],
        },
        active: {
          text: colors.interactive.solid[intent].active,
          background: colors.interactive.tonal[intent][1],
        },
        focus: {
          text: colors.interactive.solid[intent].default,
          border: colors.interactive.solid[intent].default,
          background: 'transparent',
        },
      };
    }
    case 'border':
      if (intent === 'neutral') {
        return {
          default: {
            text: colors.text.slightlyMuted,
            border: colors.interactive.tonal.neutral[2],
            background: 'transparent',
          },
          hover: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[1],
          },
          active: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[2],
          },
          focus: {
            text: colors.text.slightlyMuted,
            border: colors.text.slightlyMuted,
            background: colors.interactive.tonal.neutral[0],
          },
        };
      } else {
        return {
          default: {
            text: colors.interactive.solid[intent].default,
            border: colors.interactive.solid[intent].default,
            background: 'transparent',
          },
          hover: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].hover,
          },
          active: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].active,
          },
          focus: {
            text: colors.text.primaryInverse,
            border: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].default,
          },
        };
      }
  }
};

const horizontalPadding = <E extends React.ElementType>(
  props: ButtonProps<E>
) => {
  if (props.compact) {
    return 4;
  }
  if (props.inputAlignment) {
    return 16;
  }
  if (props.size === 'small') {
    return 8;
  }
  return 32;
};

const size = <E extends React.ElementType>(props: ThemedButtonProps<E>) => {
  const borderWidth = 1.5;
  const hp = horizontalPadding(props);
  const commonStyles = {
    borderWidth: `${borderWidth}px`,
    padding: `0 ${hp - borderWidth}px`,
  };
  switch (props.size) {
    case 'small':
      return {
        ...commonStyles,
        minHeight: '24px',
        fontSize: '12px',
        lineHeight: '16px',
        letterSpacing: '0.15px',
      };
    case 'large':
      return {
        ...commonStyles,
        minHeight: '40px',
        fontSize: '14px',
        lineHeight: '20px',
        letterSpacing: '0.175px',
      };
    case 'extra-large':
      return {
        ...commonStyles,
        minHeight: '44px',
        fontSize: '16px',
        lineHeight: '24px',
        letterSpacing: '0.2px',
      };
    case 'medium':
      return {
        ...commonStyles,
        minHeight: '32px',
        fontSize: '12px',
        lineHeight: '16px',
        letterSpacing: '0.15px',
      };
  }
};

const block = (props: { block?: boolean }) =>
  props.block
    ? {
        width: '100%',
      }
    : null;

const textTransform = (props: { textTransform?: string }) =>
  props.textTransform ? { textTransform: props.textTransform } : null;

const StyledButton = styled.button.withConfig({
  shouldForwardProp: (prop, target) =>
    !['compact'].includes(prop) && defaultValidatorFn(prop, target),
})<{ fill: ButtonFill; size: ButtonSize; intent: ButtonIntent }>`
  line-height: 1.5;
  margin: 0;
  display: inline-flex;
  justify-content: center;
  align-items: center;
  box-sizing: border-box;

  // The designs specify border as 1.5px. Unfortunately, it causes the text to
  // jump left and right when we show or hide it, even if we compensate exactly
  // 1.5px with padding. It's just some weird quirk of layout. So instead of
  // that, we keep the border always 1px thick, and instead manipulate its
  // color, switching between transparent and an actual color.
  border-style: solid;
  border-color: transparent;
  border-radius: 4px;

  cursor: pointer;
  font-family: inherit;
  font-weight: ${props => props.theme.fontWeights.bold};
  outline: none;
  position: relative;
  text-align: center;
  text-decoration: none;

  // Carefully pick what we animate. If you ever change it, make sure that
  // size-related animations don't cause the layout outside the button to shake
  // because of rounding errors when switching the button state (e.g. focusing
  // it). This could be especially visible when animating horizontal padding and
  // border width together. It would cause table borders on the storybook page
  // to shake.
  transition:
    background-color 0.1s,
    border-color 0.1s,
    outline-color 0.1s,
    box-shadow 0.1s;
  -webkit-font-smoothing: antialiased;

  ${themedStyles}
`;

export const ButtonPrimary = <E extends React.ElementType = 'button'>(
  props: ButtonProps<E>
) => <Button fill="filled" intent="primary" {...props} />;
export const ButtonSecondary = <E extends React.ElementType = 'button'>(
  props: ButtonProps<E>
) => <Button fill="filled" intent="neutral" {...props} />;
export const ButtonBorder = <E extends React.ElementType = 'button'>(
  props: ButtonProps<E>
) => <Button fill="border" intent="neutral" {...props} />;
export const ButtonWarning = <E extends React.ElementType = 'button'>(
  props: ButtonProps<E>
) => <Button fill="filled" intent="danger" {...props} />;
export const ButtonWarningBorder = <E extends React.ElementType = 'button'>(
  props: ButtonProps<E>
) => <Button fill="border" intent="danger" {...props} />;
export const ButtonText = <E extends React.ElementType = 'button'>(
  props: ButtonProps<E>
) => <Button fill="minimal" intent="neutral" {...props} />;
