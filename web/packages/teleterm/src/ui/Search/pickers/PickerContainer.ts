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

export const PickerContainer = styled.div`
  display: flex;
  flex-direction: column;
  position: fixed;
  left: 0;
  right: 0;
  margin-left: auto;
  margin-right: auto;
  box-sizing: border-box;
  z-index: 1000;
  font-size: 12px;
  color: ${props => props.theme.colors.text.main};
  background: ${props => props.theme.colors.levels.elevated};
  box-shadow: ${props => props.theme.boxShadow[1]};
  border-radius: ${props => props.theme.radii[2]}px;
  text-shadow: none;
  // Prevents inner items from covering the border on rounded corners.
  overflow: hidden;

  // Account for border.
  margin-top: -1px;

  // These values are adjusted so that the cluster selector and search input
  // are minimally covered by the picker.
  max-width: 660px;
  width: 76%;
`;
