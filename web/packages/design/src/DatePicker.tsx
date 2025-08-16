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
  height: 342px;

  .rdp-root {
    /* color of the next/previous month buttons */
    .rdp-chevron {
      fill: ${p => p.theme.colors.text.main};
    }
    --rdp-disabled-opacity: 0.25;
    padding-left: ${p => p.theme.space[3]}px;
    padding-right: ${p => p.theme.space[3]}px;
    /* Accent color for the background of selected days. */
    --rdp-accent-color: ${p => p.theme.colors.spotBackground[2]};
    --rdp-today-color: ${p => p.theme.colors.text.main};
    --rdp-accent-background-color: ${p => p.theme.colors.spotBackground[2]};

    /* color of the bar between from and to */
    --rdp-range_middle-background-color: ${p =>
      p.theme.colors.spotBackground[2]};
    --rdp-range_start-date-background-color: ${p =>
      p.theme.colors.spotBackground[2]};
    --rdp-range_start-end-background-color: ${p =>
      p.theme.colors.spotBackground[2]};
    --rdp-selected-border: 2px solid ${p => p.theme.colors.spotBackground[2]};
  }
`;
