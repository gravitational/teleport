import React from 'react';
import styled, { keyframes } from 'styled-components';

import {
  ServerIcon,
  ServerLightGreen1,
} from 'teleport/Discover/Desktop/DiscoverDesktops/ServerIcon';

const ripple = keyframes`
  0% {
    box-shadow: 0 0 0 0 rgba(255, 255, 255, 0),
    0 0 0 40px rgba(255, 255, 255, 0.18);
  }
  100% {
    box-shadow: 0 0 0 40px rgba(255, 255, 255, 0.18),
    0 0 0 40px rgba(204, 233, 251, 0);
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
  animation: ${ripple} 1.5s linear infinite;
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
    background: rgba(34, 44, 89, 1);
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
