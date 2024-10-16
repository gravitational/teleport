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

import { ButtonText } from 'design';

// TODO(bl-nero): These buttons are used in a situation where there's an error
// message and the button is responsible for retrying the operation. Convert
// this to the new alert-with-button UI pattern.
export const ButtonBlueText = styled(ButtonText)`
  color: ${({ theme }) => theme.colors.buttons.link.default};
  font-weight: normal;
  padding: 0;
  font-size: inherit;
  min-height: auto;
`;
