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

interface UserIconProps {
  letter: string;
}

export function UserIcon(props: UserIconProps) {
  return <Circle>{props.letter.toLocaleUpperCase()}</Circle>;
}

const Circle = styled.span`
  border-radius: 50%;
  color: ${props => props.theme.colors.buttons.primary.text};
  background: ${props => props.theme.colors.buttons.primary.default};
  height: 24px;
  width: 24px;
  display: flex;
  flex-shrink: 0;
  justify-content: center;
  align-items: center;
  overflow: hidden;
`;
