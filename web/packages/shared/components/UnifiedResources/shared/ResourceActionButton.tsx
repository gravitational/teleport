/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

/**
 * Wrapper to apply shared styles across action buttons that does
 * not require request. This is to help distinguish between
 * requestable and non requestable buttons.
 */
export const ResourceActionButtonWrapper = styled.div<{
  requiresRequest: boolean;
}>`
  line-height: 0;

  ${p =>
    !p.requiresRequest &&
    css`
      button,
      a {
        background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
        border: none;
      }
    `}
`;
