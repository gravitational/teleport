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

import { ComponentProps } from 'react';
import styled, { StyleFunction } from 'styled-components';

// TODO(ravicious): Put MenuList definition next to Menu once Menu is rewritten in TypeScript.
const MenuList = styled.div.attrs({ role: 'menu' })<{
  menuListCss?: StyleFunction<ComponentProps<'div'>>;
}>`
  background-color: ${props => props.theme.colors.levels.elevated};
  border-radius: 4px;
  box-shadow: ${props => props.theme.boxShadow[0]};
  box-sizing: border-box;
  max-height: calc(100% - 96px);
  overflow: hidden;
  overflow-y: auto;
  position: relative;
  padding: 0;

  ${props => props.menuListCss && props.menuListCss(props)}
`;

export default MenuList;
