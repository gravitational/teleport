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

import styled, { keyframes } from 'styled-components';

import Box, { BoxProps } from 'design/Box';

const loading = keyframes`
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(100%);
  }
`;

const ShimmerWrapper = styled.div`
  width: 100%;
  height: 100%;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  border-radius: ${props => props.theme.radii[2]}px;
  overflow: hidden;
  position: relative;
`;

const Shimmer = styled.div`
  width: 100%;
  height: 100%;
  background: linear-gradient(
    90deg,
    transparent 25%,
    ${props => props.theme.colors.spotBackground[0]} 50%,
    transparent 75%
  );
  background-size: 200% 100%;
  animation: ${loading} 1.5s infinite;
`;

export const ShimmerBox = (props: BoxProps) => {
  return (
    <Box {...props}>
      <ShimmerWrapper>
        <Shimmer />
      </ShimmerWrapper>
    </Box>
  );
};
