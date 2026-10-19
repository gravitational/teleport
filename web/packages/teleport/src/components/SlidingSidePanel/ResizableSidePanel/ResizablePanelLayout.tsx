/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

export const ResizablePanelLayout = styled(Flex)<{
  $withHorizontalScroll?: boolean;
}>`
  flex: 1;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow-x: ${p => (p.$withHorizontalScroll ? 'auto' : 'hidden')};
  overflow-y: hidden;
`;

export const ResizableSidePanelMainContent = styled(Flex)<{
  $minWidth: number | string;
}>`
  flex: 1;
  min-width: ${p =>
    typeof p.$minWidth === 'number' ? `${p.$minWidth}px` : p.$minWidth};
  flex-direction: column;
  overflow: auto;
`;
