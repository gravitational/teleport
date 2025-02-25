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

export const LinearProgress = ({
  transparentBackground = false,
  absolute = true,
  hidden = false,
}) => {
  return (
    <Wrapper $absolute={absolute} $hidden={hidden}>
      <StyledProgress $transparentBackground={transparentBackground}>
        <div className="parent-bar-2" />
      </StyledProgress>
    </Wrapper>
  );
};

const Wrapper = styled.div<{ $absolute: boolean; $hidden: boolean }>`
  ${props =>
    props.$absolute &&
    `
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    `}
  ${props => props.$hidden && `visibility: hidden;`}
`;

const StyledProgress = styled.div<{ $transparentBackground: boolean }>`
  position: relative;
  overflow: hidden;
  display: block;
  height: 1px;
  z-index: 0;
  background-color: ${props =>
    props.$transparentBackground
      ? 'transparent'
      : props.theme.colors.levels.surface};

  .parent-bar-2 {
    position: absolute;
    left: 0;
    bottom: 0;
    top: 0;
    transition: transform 0.2s linear;
    transform-origin: left;
    background-color: #1976d2;
    animation: animation-linear-progress 2s cubic-bezier(0.165, 0.84, 0.44, 1)
      0.1s infinite;
  }

  @keyframes animation-linear-progress {
    0% {
      left: -300%;
      right: 100%;
    }

    60% {
      left: 107%;
      right: -8%;
    }

    100% {
      left: 107%;
      right: -8%;
    }
  }
`;
