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

const MenuItemIcon = styled.span`
  font-size: ${props => props.theme.fontSizes[4]}px;
  margin-right: ${props => props.theme.space[2]}px;
  &:hover,
  &:focus {
    color: ${props => props.theme.colors.text.main};
  }
  svg {
    height: ${props => (props.size && `${props.size}px`) || '18px'};
    height: ${props => (props.size && `${props.size}px`) || '18px'};
  }
`;

MenuItemIcon.displayName = 'MenuItemIcon';

export default MenuItemIcon;
