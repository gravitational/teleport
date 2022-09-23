import React from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Desktop/ConnectTeleport/PingTeleportContext';
import { JoinTokenProvider } from 'teleport/Discover/Desktop/ConnectTeleport/JoinTokenContext';
import {
  PING_INTERVAL,
  PING_TIMEOUT,
  SCRIPT_TIMEOUT,
} from 'teleport/Discover/Desktop/config';

interface DesktopWrapperProps {
  children: React.ReactNode;
}

export function DesktopWrapper(props: DesktopWrapperProps) {
  return (
    <JoinTokenProvider timeout={SCRIPT_TIMEOUT}>
      <PingTeleportProvider timeout={PING_TIMEOUT} interval={PING_INTERVAL}>
        {props.children}
      </PingTeleportProvider>
    </JoinTokenProvider>
  );
}
