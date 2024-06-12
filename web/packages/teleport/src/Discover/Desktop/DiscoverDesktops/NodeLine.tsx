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

import React from 'react';
import styled, { keyframes } from 'styled-components';

interface NodeLineProps {
  children?: React.ReactNode;
  width: number;
  height: number;
}

const appear = keyframes`
  from {
    width: 0;
  }
  to {
    width: 260px;
  }
`;

const NodeLineContainer = styled.div`
  position: absolute;
  top: 43px;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
  overflow: hidden;
  animation: ${appear} 1s ease-in forwards;

  svg {
    path {
      fill: none;
    }
  }
`;

const line = keyframes`
  0% {
    stroke-dashoffset: -250;
  }

  100% {
    stroke-dashoffset: 0;
  }
`;

export const StyledSVG = styled.svg`
  position: absolute;
  z-index: 1;

  path {
    stroke: #278348;
    stroke-width: 4;
    fill: none;
  }
`;

export const AnimatedStyledSVG = styled(StyledSVG)`
  stroke-dasharray: 5, 20;
  stroke-dashoffset: 0;
  z-index: 2;
  animation: ${line} 5s cubic-bezier(0.78, 0.11, 0.27, 0.94) alternate infinite
    0.6s;

  path {
    stroke: #32c842;
  }
`;

export function NodeLine(props: NodeLineProps) {
  return (
    <NodeLineContainer
      style={{ width: props.width, height: props.height, left: -props.width }}
    >
      <StyledSVG width={props.width} height={props.height}>
        {props.children}
      </StyledSVG>
      <AnimatedStyledSVG width={props.width} height={props.height}>
        {props.children}
      </AnimatedStyledSVG>
    </NodeLineContainer>
  );
}
