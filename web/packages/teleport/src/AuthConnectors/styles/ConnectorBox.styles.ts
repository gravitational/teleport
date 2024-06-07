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

import { Box, Flex } from 'design';
import styled from 'styled-components';

export const ConnectorBox = styled(Box)`
  display: flex;
  flex-direction: column;
  font-family: ${props => props.theme.font};
  width: 320px;
  padding: ${p => p.theme.space[4]}px;
  margin: ${p => p.theme.space[3]}px ${p => p.theme.space[2]}px;
  background: transparent;
  transition: all 0.3s;
  border-radius: ${props => props.theme.radii[2]}px;
  min-height: 190px;
  border: ${props => props.theme.borders[2]}
    ${p => p.theme.colors.spotBackground[0]};

  &:hover,
  &:focus {
    border: ${props => props.theme.borders[2]}
      ${p => p.theme.colors.spotBackground[2]};
    background: ${p => p.theme.colors.spotBackground[0]};
    box-shadow: ${p => p.theme.boxShadow[3]};
    cursor: pointer;
  }

  &:disabled {
    cursor: not-allowed;
    color: inherit;
    font-family: inherit;
    outline: none;
    position: relative;
    text-align: center;
    text-decoration: none;
    opacity: 0.24;
    box-shadow: none;
  }
`;

export const ResponsiveConnector = styled(Flex)`
  position: relative;
  box-shadow: ${p => p.theme.boxShadow[5]};
  width: 240px;
  height: 240px;
  border-radius: ${props => props.theme.radii[2]}px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: ${props => props.theme.space[5]}px;
  @media screen and (max-width: ${props => props.theme.breakpoints.tablet}px) {
    width: 100%;
  }
`;
