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

interface TooltipPositionProps {
  position: 'left' | 'middle' | 'right';
}

const tooltipPosition = (props: TooltipPositionProps) => {
  if (props.position === 'right') {
    return {
      right: '2px',
    };
  }
  if (props.position === 'middle') {
    return {
      left: '50%',
      transform: 'translateX(-50%)',
    };
  }
  return {
    left: '2px',
  };
};
const tooltipCaretPosition = (props: TooltipPositionProps) => {
  if (props.position === 'right') {
    return {
      right: '10px',
    };
  }
  if (props.position === 'middle') {
    return {
      left: '50%',
      transform: 'translateX(-50%)',
    };
  }
  return {
    left: '10px',
  };
};

export const Tooltip = styled.div<TooltipPositionProps>`
  ${tooltipPosition};

  position: absolute;
  white-space: nowrap;
  pointer-events: none;
  top: 40px;
  z-index: 999;
  background: rgba(0, 0, 0, 0.8);
  color: white;
  border-radius: 7px;
  padding: 5px 8px;

  &:after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
    border-width: 0 7px 7px 7px;
    border-color: transparent transparent rgba(0, 0, 0, 0.8) transparent;
    top: -7px;

    ${tooltipCaretPosition};
  }
`;
