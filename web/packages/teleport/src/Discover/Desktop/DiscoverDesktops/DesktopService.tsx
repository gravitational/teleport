/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
