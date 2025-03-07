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

// NOTE: This MainContainer component is imported in multiple places and then
// modified using styled-components. If it's exported from the Main/index.ts
// or included in the Main.tsx file there is a circular dependency that causes
// odd issues around the MainContainer component being available at certain
// times.
export const MainContainer = styled.div`
  display: flex;
  flex: 1;
  --sidenav-width: 84px;
  --sidenav-panel-width: 264px;
  overflow: hidden;
  margin-top: ${p => p.theme.topBarHeight[0]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    margin-top: ${p => p.theme.topBarHeight[1]}px;
  }
`;
