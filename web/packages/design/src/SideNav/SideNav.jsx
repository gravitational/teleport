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

const SideNav = styled.nav`
  background: ${props => props.theme.colors.levels.surface};
  border-right: 1px solid ${props => props.theme.colors.levels.sunken};
  min-width: 240px;
  width: 240px;
  overflow: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
`;

SideNav.displayName = 'SideNav';

export default SideNav;
