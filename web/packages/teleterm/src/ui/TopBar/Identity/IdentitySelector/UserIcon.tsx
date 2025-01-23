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

import { MouseEvent } from 'react';
import styled from 'styled-components';

import {
  ProfileColor,
  profileColorMapping,
} from 'teleterm/ui/services/workspacesService';

export function UserIcon(props: {
  letter: string;
  /** If not provided, a default neutral color is rendered. */
  color?: ProfileColor;
  className?: string;
  onClick?(e: MouseEvent<HTMLSpanElement>): void;
}) {
  return (
    <Circle
      onClick={props.onClick}
      className={props.className}
      color={profileColorMapping[props.color]}
    >
      {props.letter.toLocaleUpperCase()}
    </Circle>
  );
}

const Circle = styled.span<{ color?: string }>`
  border-radius: 50%;
  color: ${props =>
    props.color ? 'white' : props.theme.colors.interactive.solid.primary};
  background: ${props =>
    props.color || props.theme.colors.interactive.tonal.neutral[1]};
  height: 30px;
  width: 30px;
  display: flex;
  font-weight: 500;
  justify-content: center;
  align-items: center;
  box-shadow: rgba(0, 0, 0, 0.15) 0 1px 3px;
`;
