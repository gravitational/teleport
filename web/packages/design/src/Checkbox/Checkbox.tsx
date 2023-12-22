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

import styled from 'styled-components';

import { Flex } from 'design';
import { space } from 'design/system';

export const CheckboxWrapper = styled(Flex)`
  padding: 8px;
  margin-bottom: 4px;
  width: 300px;
  align-items: center;
  border: 1px solid ${props => props.theme.colors.spotBackground[1]};
  border-radius: 8px;

  &.disabled {
    pointer-events: none;
    opacity: 0.5;
  }
`;

export const CheckboxInput = styled.input`
  margin-right: 10px;
  accent-color: ${props => props.theme.colors.brand};

  &:hover {
    cursor: pointer;
  }

  ${space}
`;

// TODO (avatus): Make this the default checkbox
export const StyledCheckbox = styled.input.attrs({ type: 'checkbox' })`
  // reset the appearance so we can style the background
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
  width: 16px;
  height: 16px;
  border: 1px solid ${props => props.theme.colors.text.muted};
  border-radius: ${props => props.theme.radii[1]}px;
  background: transparent;
  position: relative;

  &:checked {
    border: 1px solid ${props => props.theme.colors.brand};
    background-color: ${props => props.theme.colors.brand};
  }

  &:hover {
    cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
  }

  &::before {
    content: '';
    display: block;
  }

  &:checked::before {
    content: '✓';
    color: ${props => props.theme.colors.levels.deep};
    position: absolute;
    right: 1px;
    top: -1px;
  }
`;
