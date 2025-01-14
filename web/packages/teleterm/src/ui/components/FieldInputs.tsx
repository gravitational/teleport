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

import FieldInput from 'shared/components/FieldInput';

export const ConfigFieldInput = styled(FieldInput).attrs({ size: 'small' })`
  input {
    &:invalid,
    &:invalid:hover {
      border-color: ${props =>
        props.theme.colors.interactive.solid.danger.default};
    }
  }
`;

export const PortFieldInput = styled(ConfigFieldInput).attrs({
  type: 'number',
  min: 1,
  max: 65535,
  // Without a min width, the stepper controls end up being to close to a long port number such
  // as 65535. minWidth instead of width allows the field to grow with the label, so that e.g.
  // a custom label of "Local Port (optional)" is displayed on a single line.
  minWidth: '110px',
})``;
