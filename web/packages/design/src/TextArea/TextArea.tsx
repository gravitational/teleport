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

import React, {
  forwardRef,
  HTMLAttributes,
  HTMLInputAutoCompleteAttribute,
} from 'react';
import styled, { CSSObject } from 'styled-components';
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

import { Theme } from 'design/theme/themes/types';

export type TextAreaSize = 'large' | 'medium' | 'small';

export interface TextAreaProps
  extends ColorProps,
    SpaceProps,
    WidthProps,
    HeightProps {
  size?: TextAreaSize;
  hasError?: boolean;
  resizable?: boolean;

  // TextArea element attributes
  autoFocus?: boolean;
  disabled?: boolean;
  id?: string;
  name?: string;
  readOnly?: boolean;
  value?: string;
  defaultValue?: string;
  placeholder?: string;
  autoComplete?: HTMLInputAutoCompleteAttribute;
  spellCheck?: boolean;
  style?: React.CSSProperties;

  'aria-invalid'?: HTMLAttributes<'textarea'>['aria-invalid'];
  'aria-describedby'?: HTMLAttributes<'textarea'>['aria-describedby'];

  onChange?: React.ChangeEventHandler<HTMLInputElement>;
  onKeyPress?: React.KeyboardEventHandler<HTMLInputElement>;
  onKeyDown?: React.KeyboardEventHandler<HTMLInputElement>;
  onFocus?: React.FocusEventHandler<HTMLInputElement>;
  onBlur?: React.FocusEventHandler<HTMLInputElement>;
  onClick?: React.MouseEventHandler<HTMLInputElement>;
}

export const textAreaGeometry: {
  [s in TextAreaSize]: {
    height: number;
    typography: keyof Theme['typography'];
  };
} = {
  large: {
    height: 96,
    typography: 'body1',
  },
  medium: {
    height: 84,
    typography: 'body2',
  },
  small: {
    height: 76,
    typography: 'body3',
  },
};

export const TextArea = forwardRef<HTMLTextAreaElement, TextAreaProps>(
  ({ size = 'medium', ...rest }, ref) => (
    <StyledTextArea ref={ref} taSize={size} {...rest} />
  )
);

type StyledTextAreaProps = Omit<TextAreaProps, 'size'> & {
  taSize: TextAreaSize;
};

const StyledTextArea = styled.textarea<StyledTextAreaProps>`
  appearance: none;
  border: 1px solid;
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 4px;
  box-sizing: border-box;
  display: block;
  min-height: 50px;
  height: ${props => textAreaGeometry[props.taSize].height}px;
  padding: 8px 16px;
  outline: none;
  width: 100%;
  background-color: transparent;
  color: ${props => props.theme.colors.text.main};

  ${props => props.theme.typography[textAreaGeometry[props.taSize].typography]}

  &:hover {
    border: 1px solid ${props => props.theme.colors.text.muted};
  }

  &:focus-visible {
    border-color: ${props =>
      props.theme.colors.interactive.solid.primary.default};
  }

  &::placeholder {
    color: ${props => props.theme.colors.text.muted};
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
  ${resize}
`;

function error({
  hasError,
  theme,
}: Pick<TextAreaProps, 'hasError'> & {
  theme: any;
}) {
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

function resize({ resizable }: Pick<TextAreaProps, 'resizable'>): CSSObject {
  return { resize: resizable ? 'vertical' : 'none' };
}
