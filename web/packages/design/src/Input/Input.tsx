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

import React, {
  forwardRef,
  HTMLAttributes,
  HTMLInputAutoCompleteAttribute,
} from 'react';
import styled, { css, useTheme } from 'styled-components';
import {
  color,
  ColorProps,
  height,
  HeightProps,
  space,
  SpaceProps,
  width,
  WidthProps,
} from 'styled-system';

import Box from 'design/Box';
import * as Icon from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { Theme } from 'design/theme/themes/types';

export type InputSize = 'large' | 'medium' | 'small';
export type InputType =
  | 'email'
  | 'text'
  | 'password'
  | 'number'
  | 'date'
  | 'week';
export type InputMode = HTMLAttributes<'input'>['inputMode'];

interface InputProps extends ColorProps, SpaceProps, WidthProps, HeightProps {
  size?: InputSize;
  hasError?: boolean;
  icon?: React.ComponentType<IconProps>;

  // Input properties
  autoFocus?: boolean;
  disabled?: boolean;
  id?: string;
  name?: string;
  readOnly?: boolean;
  role?: string;
  type?: InputType;
  value?: string;
  defaultValue?: string;
  placeholder?: string;
  min?: number;
  max?: number;
  autoComplete?: HTMLInputAutoCompleteAttribute;
  inputMode?: InputMode;
  spellCheck?: boolean;
  style?: React.CSSProperties;
  required?: boolean;

  'aria-invalid'?: HTMLAttributes<'input'>['aria-invalid'];
  'aria-describedby'?: HTMLAttributes<'input'>['aria-describedby'];

  onChange?: React.ChangeEventHandler<HTMLInputElement>;
  onKeyPress?: React.KeyboardEventHandler<HTMLInputElement>;
  onKeyDown?: React.KeyboardEventHandler<HTMLInputElement>;
  onFocus?: React.FocusEventHandler<HTMLInputElement>;
  onBlur?: React.FocusEventHandler<HTMLInputElement>;
  onClick?: React.MouseEventHandler<HTMLInputElement>;
}

export const inputGeometry: {
  [s in InputSize]: {
    height: number;
    iconSize: number;
    horizontalGap: number;
    typography: keyof Theme['typography'];
  };
} = {
  large: {
    height: 48,
    iconSize: 20,
    horizontalGap: 12,
    typography: 'body1',
  },
  medium: {
    height: 40,
    iconSize: 18,
    horizontalGap: 8,
    typography: 'body2',
  },
  small: {
    height: 32,
    iconSize: 16,
    horizontalGap: 8,
    typography: 'body3',
  },
};

const borderSize = 1;
const baseHorizontalPadding = 16;
const errorIconHorizontalPadding = 8;

function error({ hasError, theme }: { hasError?: boolean; theme: Theme }) {
  if (!hasError) {
    return;
  }

  return {
    borderColor: theme.colors.interactive.solid.danger.default,
    '&:hover': {
      borderColor: theme.colors.interactive.solid.danger.default,
    },
  };
}

function padding({
  hasError,
  hasIcon,
  inputSize,
}: {
  hasError?: boolean;
  hasIcon: boolean;
  inputSize: InputSize;
}) {
  const { iconSize, horizontalGap } = inputGeometry[inputSize];
  const paddingRight = hasError
    ? errorIconHorizontalPadding + horizontalGap + iconSize
    : baseHorizontalPadding;
  const paddingLeft = hasIcon
    ? baseHorizontalPadding + horizontalGap + iconSize
    : baseHorizontalPadding;
  return css`
    padding: 0 ${paddingRight}px 0 ${paddingLeft}px;
  `;
}

const Input = forwardRef<HTMLInputElement, InputProps>((props, ref) => {
  const {
    size = 'medium',
    hasError,
    icon: IconComponent,

    autoFocus,
    disabled,
    id,
    name,
    readOnly,
    role,
    type,
    value,
    defaultValue,
    placeholder,
    min,
    max,
    autoComplete,
    inputMode,
    spellCheck,
    style,
    required,

    'aria-invalid': ariaInvalid,
    'aria-describedby': ariaDescribedBy,

    onChange,
    onKeyPress,
    onKeyDown,
    onFocus,
    onBlur,
    onClick,
    ...wrapperProps
  } = props;
  const theme = useTheme();
  const { iconSize } = inputGeometry[size];
  return (
    <InputWrapper inputSize={size} {...wrapperProps}>
      {IconComponent && (
        <IconWrapper>
          <IconComponent
            role="graphics-symbol"
            size={iconSize}
            color={
              disabled
                ? theme.colors.text.disabled
                : theme.colors.text.slightlyMuted
            }
          />
        </IconWrapper>
      )}
      <StyledInput
        ref={ref}
        hasIcon={!!IconComponent}
        inputSize={size}
        {...{
          hasError,

          autoFocus,
          disabled,
          id,
          name,
          readOnly,
          role,
          type,
          value,
          defaultValue,
          placeholder,
          min,
          max,
          autoComplete,
          inputMode,
          spellCheck,
          style,
          required,

          'aria-invalid': ariaInvalid,
          'aria-describedby': ariaDescribedBy,

          onChange,
          onKeyPress,
          onKeyDown,
          onFocus,
          onBlur,
          onClick,
        }}
      />
      {hasError && (
        <ErrorIcon size={iconSize} role="graphics-symbol" aria-label="Error" />
      )}
    </InputWrapper>
  );
});

const InputWrapper = styled(Box)<{ inputSize: InputSize }>`
  position: relative;
  height: ${props => inputGeometry[props.inputSize].height}px;
`;

const IconWrapper = styled.div`
  position: absolute;
  left: ${borderSize + baseHorizontalPadding}px;
  top: 0;
  bottom: 0;
  display: flex; // For vertical centering.
`;

const StyledInput = styled.input<{ hasIcon: boolean; inputSize: InputSize }>`
  appearance: none;
  border: ${borderSize}px solid;
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 4px;
  box-sizing: border-box;
  display: block;
  height: 100%;
  outline: none;
  width: 100%;
  background-color: transparent;
  color: ${props => props.theme.colors.text.main};

  ${props => props.theme.typography[inputGeometry[props.inputSize].typography]}
  ${padding}

  &:hover {
    border: 1px solid ${props => props.theme.colors.text.muted};
  }

  &:focus-visible {
    border-color: ${props =>
      props.theme.colors.interactive.solid.primary.default};
  }

  &::-ms-clear {
    display: none;
  }

  &::placeholder {
    color: ${props => props.theme.colors.text.muted};
    opacity: 1;
  }

  &:disabled::placeholder {
    color: ${props => props.theme.colors.text.disabled};
    opacity: 1;
  }

  &:read-only {
    cursor: not-allowed;
  }

  &:disabled {
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[0]};
    color: ${props => props.theme.colors.text.disabled};
    border-color: transparent;
  }

  ${color}
  ${space}
  ${width}
  ${height}
  ${error}
`;

const ErrorIcon = styled(Icon.WarningCircle)`
  position: absolute;
  right: ${errorIconHorizontalPadding + borderSize}px;
  color: ${props => props.theme.colors.interactive.solid.danger.default};
  top: 0;
  bottom: 0;
`;

export default Input;
