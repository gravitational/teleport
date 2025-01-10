/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Box } from 'design';

export enum ItemStatus {
  Success,
  Warning,
  Error,
}

export const StatusLight = styled(Box)<{ status: ItemStatus }>`
  border-radius: 50%;
  margin-right: ${props => props.theme.space[2]}px;
  width: 8px;
  height: 8px;
  background-color: ${({ status, theme }) => {
    if (status === ItemStatus.Success) {
      return theme.colors.success.main;
    }
    if (status === ItemStatus.Error) {
      return theme.colors.error.main;
    }
    if (status === ItemStatus.Warning) {
      return theme.colors.warning;
    }
    return theme.colors.grey[300]; // Unknown
  }};
`;
