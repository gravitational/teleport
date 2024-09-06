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

import { ButtonIcon } from 'design';

export const StyledArrowBtn = styled(ButtonIcon)`
  svg {
    font-size: ${props => props.theme.fontSizes[5]}px;
  }
  svg:before {
    // arrow icons have some padding that makes them look slightly off-center, padding compensates it
    padding-left: 1px;
  }
`;

export const StyledFetchMoreBtn = styled.button`
  color: ${props => props.theme.colors.buttons.link.default};
  background: none;
  text-decoration: underline;
  text-transform: none;
  outline: none;
  border: none;
  font-weight: bold;
  line-height: 0;
  font-size: ${props => props.theme.fontSizes[1]}px;

  &:hover,
  &:focus {
    cursor: pointer;
  }

  &:disabled {
    color: ${props => props.theme.colors.text.disabled};
    cursor: wait;
  }
`;
