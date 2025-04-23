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

import { Card } from 'design';

export const OnboardCard = styled(Card)<{ center?: boolean }>`
  width: 600px;
  padding: ${props => props.theme.space[4]}px;
  text-align: ${props => (props.center ? 'center' : 'left')};
  margin: ${props => props.theme.space[3]}px auto
    ${props => props.theme.space[3]}px auto;
  overflow-y: auto;

  @media screen and (max-width: 800px) {
    width: auto;
    margin: 20px;
  }

  @media screen and (max-height: 760px) {
    height: calc(100vh - 250px);
  }
`;
