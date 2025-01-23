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

export const StaticListItem = styled.li`
  white-space: nowrap;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  outline: none;
  position: relative;
  font-size: 14px;
  padding: 0 16px;
  height: 34px;
  background: inherit;
  border: none;
  border-radius: 4px;
`;

export const ListItem = styled(StaticListItem).attrs({ tabIndex: 0 })<{
  isActive?: boolean;
}>`
  cursor: pointer;
  background: ${props =>
    props.isActive ? props.theme.colors.interactive.tonal.neutral[0] : null};

  &:focus-visible {
    outline: 1px solid ${props => props.theme.colors.text.muted};
    background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  }
  &:hover {
    outline: 1px solid
      ${props => props.theme.colors.interactive.tonal.neutral[0]};
    background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  }
`;
