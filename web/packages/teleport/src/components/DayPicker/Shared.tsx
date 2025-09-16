/*
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

import { Flex } from 'design';

export const StyledDateRange = styled(Flex)`
  position: relative;
  box-sizing: border-box;
  background-color: ${p => p.theme.colors.levels.popout};
  box-shadow: ${p => p.theme.boxShadow[3]};
  border-radius: ${p => p.theme.radii[2]}px;

  // This is to prevent jumping from resize when day picker is used
  // inside a dialog from longer months vs shorter months
  height: 336px;

  .rdp {
    /* Accent color for the background of selected days. */
    --rdp-accent-color: ${p => p.theme.colors.spotBackground[2]};

    /* Background color for the hovered/focused elements. */
    --rdp-background-color: ${p => p.theme.colors.spotBackground[0]};

    /* Color of selected day text */
    --rdp-selected-color: ${p => p.theme.colors.text.main};

    --rdp-outline: 2px solid var(--rdp-accent-color); /* Outline border for focused elements */
    --rdp-outline-selected: 3px solid var(--rdp-accent-color); /* Outline border for focused _and_ selected elements */
  }
`;
