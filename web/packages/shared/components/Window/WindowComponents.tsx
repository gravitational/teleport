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

export const WindowContainer = styled.div`
  border-radius: 5px;
  width: 100%;
  box-shadow: 0px 0px 20px 0px rgba(0, 0, 0, 0.43);
`;

export const WindowTitleBarContainer = styled.div`
  background: #040b1d;
  height: 32px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  border-top-left-radius: 5px;
  border-top-right-radius: 5px;
`;

export const WindowTitleBarButtons = styled.div`
  display: flex;
  position: absolute;
  top: 50%;
  left: 10px;
  transform: translate(0, -50%);
`;

export const WindowTitleBarButton = styled.div`
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-right: 5px;
`;

export const WindowContentContainer = styled.div`
  background: #04162c;
  height: var(--content-height, 660px);
  overflow-y: auto;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
`;

export const WindowCode = styled.div`
  font-size: 12px;
  font-family:
    Menlo,
    DejaVu Sans Mono,
    Consolas,
    Lucida Console,
    monospace;
  line-height: 20px;
  white-space: pre-wrap;
`;
