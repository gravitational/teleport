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

export const ResourceWrapper = styled(Flex)`
  flex-direction: column;
  height: 100%;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: 12px;
  gap: 8px;
  border-radius: ${props => props.theme.radii[2]}px;

  border: ${({ isSelected, invalid, theme }) => {
    if (isSelected) {
      return `1px solid ${theme.colors.brand}`;
    }
    if (invalid) {
      return `1px solid ${theme.colors.error.main}`;
    }
    return `1px solid ${theme.colors.levels.elevated}`;
  }};

  &:hover {
    background-color: ${props => props.theme.colors.spotBackground[0]};
    box-shadow: ${({ theme }) => theme.boxShadow[2]};
  }

  &:focus-within {
    background-color: ${props => props.theme.colors.spotBackground[1]};
    border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  }
`;
