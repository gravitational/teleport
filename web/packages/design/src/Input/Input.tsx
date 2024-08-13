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

import styled from 'styled-components';
import PropTypes from 'prop-types';
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

function error({ hasError, theme }) {
  if (!hasError) {
    return;
  }

  return {
    border: `2px solid ${theme.colors.error.main}`,
    '&:hover, &:focus': {
      border: `2px solid ${theme.colors.error.main}`,
    },
    padding: '10px 14px',
  };
}

interface InputProps extends ColorProps, SpaceProps, WidthProps, HeightProps {
  hasError?: boolean;
}

const Input = styled.input<InputProps>`
  appearance: none;
  border: 1px solid ${props => props.theme.colors.text.muted};
  border-radius: 4px;
  box-sizing: border-box;
  display: block;
  height: 40px;
  font-size: 16px;
  font-weight: 300;
  padding: 0 16px;
  outline: none;
  width: 100%;
  background: ${props => props.theme.colors.levels.surface};
  color: ${props => props.theme.colors.text.main};

  &:hover,
  &:focus,
  &:active {
    border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  }

  &::-ms-clear {
    display: none;
  }

  &::placeholder {
    color: ${props => props.theme.colors.text.muted};
    opacity: 1;
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
`;

Input.displayName = 'Input';

Input.propTypes = {
  placeholder: PropTypes.string,
  hasError: PropTypes.bool,
};

export default Input;
