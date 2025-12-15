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

import { MouseEvent, ReactNode } from 'react';
import styled from 'styled-components';

import {
  WorkspaceColor,
  workspaceColorMapping,
} from 'teleterm/ui/services/workspacesService';

export function UserIcon(props: {
  letter: string;
  /** If not provided, a default neutral color is rendered. */
  color?: WorkspaceColor;
  onClick?(e: MouseEvent<HTMLSpanElement>): void;
  children?: ReactNode;
  interactive?: boolean;
  size?: 'regular' | 'big';
}) {
  return (
    <Circle
      as={props.interactive ? 'button' : 'span'}
      size={props.size === 'big' ? '34px' : '30px'}
      onClick={props.onClick}
      interactive={props.interactive}
      color={workspaceColorMapping[props.color]}
    >
      {props.letter?.toLocaleUpperCase()}
      {props.children}
    </Circle>
  );
}

const Circle = styled.span<{
  color?: string;
  interactive?: boolean;
  size: string;
}>`
  position: relative;
  border-radius: 50%;
  color: ${props =>
    props.color
      ? props.theme.colors.text.primaryInverse
      : props.theme.colors.text.main};
  background: ${props =>
    props.color || props.theme.colors.interactive.tonal.neutral[1]};
  height: ${props => props.size};
  width: ${props => props.size};
  display: flex;
  font-weight: 500;
  justify-content: center;
  align-items: center;
  box-shadow: rgba(0, 0, 0, 0.15) 0 1px 3px;
  border: none;
  &:focus-visible {
    outline: 2px solid ${props => props.theme.colors.text.muted};
  }
  ${props =>
    props.interactive &&
    `
    &:hover {
      opacity: 0.9;
    }
    cursor: pointer;
    `}
`;
