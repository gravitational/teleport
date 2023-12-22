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

export const ListItem = styled.li`
  white-space: nowrap;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  cursor: pointer;
  width: 100%;
  position: relative;
  font-size: 14px;
  padding: 0 16px;
  font-weight: ${props => props.theme.regular};
  font-family: ${props => props.theme.font};
  color: ${props => props.theme.colors.text.main};
  height: 34px;
  background: inherit;
  border: none;
  border-radius: 4px;

  background: ${props =>
    props.isActive ? props.theme.colors.spotBackground[0] : null};

  &:focus,
  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
