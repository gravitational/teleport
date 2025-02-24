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

import { space, SpaceProps } from 'design/system';

interface LabelInputProps extends SpaceProps {
  hasError?: boolean;
}

export const LabelInput = styled.label<LabelInputProps>`
  color: ${props =>
    props.hasError
      ? props.theme.colors.error.main
      : props.theme.colors.text.main};
  display: block;
  width: 100%;
  margin-bottom: ${props => props.theme.space[1]}px;
  ${props => props.theme.typography.body3}
  ${space}
`;
