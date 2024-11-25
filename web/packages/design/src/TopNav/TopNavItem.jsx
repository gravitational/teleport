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

import { height, maxHeight, maxWidth, space, width } from 'design/system';

/**
 * TopNavItem
 */
const TopNavItem = styled.button`
  align-items: center;
  background: none;
  border: none;
  color: ${props =>
    props.active ? props.theme.colors.light : 'rgba(255, 255, 255, .56)'};
  cursor: pointer;
  display: inline-flex;
  font-size: 11px;
  font-weight: 600;
  height: 100%;
  margin: 0;
  outline: none;
  padding: 0 16px;
  position: relative;
  text-decoration: none;

  &:hover {
    background: ${props =>
      props.active
        ? props.theme.colors.levels.surface
        : 'rgba(255, 255, 255, .06)'};
  }

  &.active {
    background: ${props => props.theme.colors.levels.surface};
    color: ${props => props.theme.colors.light};
  }

  &.active:after {
    background-color: ${props => props.theme.colors.brand};
    content: '';
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 4px;
  }

  ${space}
  ${width}
  ${maxWidth}
  ${height}
  ${maxHeight}
`;

TopNavItem.displayName = 'TopNavItem';

export default TopNavItem;
