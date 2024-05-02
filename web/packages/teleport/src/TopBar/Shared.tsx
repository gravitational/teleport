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

export const ButtonIconContainer = styled.div<{ open: boolean }>`
  padding: 0 10px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding-left: 12px;
  padding-right: 12px;
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    padding-left: 24px;
    padding-right: 24px;
  }
  cursor: pointer;
  user-select: none;
  margin-right: 5px;

  background: ${props =>
    props.open ? props.theme.colors.spotBackground[0] : ''};
  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
