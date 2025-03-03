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

import styled, { css } from 'styled-components';

import { Box } from 'design';

export type Status = 'on' | 'off' | 'warning';

export const ConnectionsIconStatusIndicator = styled(Box)<{
  status: Status;
}>`
  position: absolute;
  top: -4px;
  right: -4px;
  z-index: 1;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  ${props => {
    const { status, theme } = props;

    if (status === 'warning') {
      return css`
        color: ${theme.colors.interactive.solid.alert.default};
        &:after {
          content: '⚠';
          font-size: 12px;

          position: absolute;
          top: -8px;
          left: -2px;
        }
      `;
    }

    const connected = status === 'on';

    const backgroundColor = connected
      ? theme.colors.interactive.solid.success.default
      : null;
    const border = connected
      ? null
      : `1px solid ${theme.colors.text.slightlyMuted}`;
    return {
      backgroundColor,
      border,
    };
  }}
`;
