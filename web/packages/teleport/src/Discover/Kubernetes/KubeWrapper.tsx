import React from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { JoinTokenProvider } from 'teleport/Discover/Shared/JoinTokenContext';

import { ResourceKind } from '../Shared';

const PING_TIMEOUT = 1000 * 60 * 5; // 5 minutes
const PING_INTERVAL = 1000 * 3; // 3 seconds
export const SCRIPT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export function KubeWrapper(props: WrapperProps) {
  return (
    <JoinTokenProvider timeout={SCRIPT_TIMEOUT}>
      <PingTeleportProvider
        timeout={PING_TIMEOUT}
        interval={PING_INTERVAL}
        resourceKind={ResourceKind.Kubernetes}
      >
        {props.children}
      </PingTeleportProvider>
    </JoinTokenProvider>
  );
}

interface WrapperProps {
  children: React.ReactNode;
}
