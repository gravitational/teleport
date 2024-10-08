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

import styled, { CSSObject } from 'styled-components';
import {
  space,
  width,
  color,
  height,
  ColorProps,
  SpaceProps,
  WidthProps,
  HeightProps,
} from 'styled-system';

export interface TextAreaProps
  extends ColorProps,
    SpaceProps,
    WidthProps,
    HeightProps {
  hasError?: boolean;
  resizable?: boolean;

  // TS: temporary handles ...styles
  [key: string]: any;
}

export const TextArea = styled.textarea<TextAreaProps>`
  appearance: none;
  border: 1px solid ${props => props.theme.colors.text.muted};
  border-radius: 4px;
  box-sizing: border-box;
  min-height: 50px;
  height: 80px;
  font-size: 16px;
  padding: 16px;
  outline: none;
  width: 100%;
  color: ${props => props.theme.colors.text.main};
  background: inherit;

  &::placeholder {
    color: ${props => props.theme.colors.text.muted};
    opacity: 1;
  }

  &:hover,
  &:focus,
  &:active {
    border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  }

  &:read-only {
    cursor: not-allowed;
  }

  &:disabled {
    color: ${props => props.theme.colors.text.disabled};
    border-color: ${props => props.theme.colors.text.disabled};
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
    border: `2px solid ${theme.colors.error.main}`,
    '&:hover, &:focus': {
      border: `2px solid ${theme.colors.error.main}`,
    },
  };
}

function resize({ resizable }: Pick<TextAreaProps, 'resizable'>): CSSObject {
  return { resize: resizable ? 'vertical' : 'none' };
}
