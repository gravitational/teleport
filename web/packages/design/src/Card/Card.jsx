/*
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

import Box from './../Box';
import { darkTheme } from './../theme';

const Card = styled(Box).attrs(props => ({ theme: darkTheme, ...props }))`
  box-shadow: ${props => props.theme.boxShadow[2]};
  border-radius: ${props => props.theme.radii[3]}px;
  background-color: ${props => props.theme.colors.levels.surface};
`;

Card.displayName = 'Card';

export default Card;
