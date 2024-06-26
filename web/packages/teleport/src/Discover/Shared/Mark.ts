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

export const Mark = styled.mark<{ light?: boolean }>`
  padding: 2px 5px;
  border-radius: 6px;
  font-family: ${props => props.theme.fonts.mono};
  font-size: 12px;
  background-color: ${props =>
    props.light ? '#d3d3d3' : props.theme.colors.spotBackground[2]};
  color: inherit;
`;
