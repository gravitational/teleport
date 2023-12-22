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

import {
  ServerIcon,
  ServerLightGreen1,
} from 'teleport/Discover/Desktop/DiscoverDesktops/ServerIcon';

const ripple = rippleColor => keyframes`
  0% {
    box-shadow: 0 0 0 0 rgba(255, 255, 255, 0),
    0 0 0 40px ${rippleColor};
  }
  100% {
    box-shadow: 0 0 0 40px ${rippleColor},
    0 0 0 40px rgba(255, 255, 255, 0);
  }
`;

const Container = styled.div`
  display: flex;
  flex-direction: column;
  position: relative;
  padding-bottom: 10px;
  justify-content: center;
  height: 82px;
`;

const Ripple = styled.div`
  animation: ${({ theme }) => ripple(theme.colors.spotBackground[2])} 1.5s
    linear infinite;
  border-radius: 50%;
  width: 100px;
  height: 100px;
  position: absolute;
  z-index: -1;
  top: 50%;
  left: 50%;
  transform: translate(-50%, calc(-50% - 10px));

  &::after {
    z-index: 0;
    border-radius: 50%;
    position: absolute;
    content: '';
    display: block;
    width: 100px;
    height: 100px;
    transform: scale(1);
  }
`;

interface DesktopServiceProps {
  desktopServiceRef: React.Ref<HTMLDivElement>;
}

export function DesktopService(props: DesktopServiceProps) {
  return (
    <Container ref={props.desktopServiceRef}>
      <Ripple />
      <ServerIcon light={<ServerLightGreen1 />} />
    </Container>
  );
}
