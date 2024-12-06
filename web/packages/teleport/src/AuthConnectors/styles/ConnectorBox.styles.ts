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

import { Box } from 'design';
import styled from 'styled-components';

export const ConnectorBox = styled(Box)`
  position: relative;
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: flex-start;
  font-family: ${props => props.theme.font};
  height: 157px;
  padding: ${p => p.theme.space[3]}px;
  background: transparent;
  transition: all 0.3s;
  border-radius: ${props => props.theme.radii[2]}px;
  border: ${props => props.theme.borders[1]}
    ${props => props.theme.colors.interactive.tonal.neutral[0]};

  &:hover,
  &:focus {
    border: ${props => props.theme.borders[1]}
      ${props => props.theme.colors.interactive.tonal.neutral[1]};
  }
`;

export const AuthConnectorsGrid = styled(Box)`
  width: 100%;
  display: grid;
  gap: ${p => p.theme.space[5]}px;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  justify-content: center;
`;
