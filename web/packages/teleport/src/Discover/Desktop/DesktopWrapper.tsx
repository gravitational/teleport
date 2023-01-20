import React from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { JoinTokenProvider } from 'teleport/Discover/Shared/JoinTokenContext';
import {
  PING_INTERVAL,
  PING_TIMEOUT,
  SCRIPT_TIMEOUT,
} from 'teleport/Discover/Desktop/config';

import { ResourceKind } from '../Shared';

interface DesktopWrapperProps {
  children: React.ReactNode;
}

export function DesktopWrapper(props: DesktopWrapperProps) {
  return (
    <JoinTokenProvider timeout={SCRIPT_TIMEOUT}>
      <PingTeleportProvider
        timeout={PING_TIMEOUT}
        interval={PING_INTERVAL}
        resourceKind={ResourceKind.Desktop}
      >
        {props.children}
      </PingTeleportProvider>
    </JoinTokenProvider>
  );
}
