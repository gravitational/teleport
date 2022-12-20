import React from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { JoinTokenProvider } from 'teleport/Discover/Shared/JoinTokenContext';
import {
  PING_INTERVAL,
  PING_TIMEOUT,
  SCRIPT_TIMEOUT,
} from 'teleport/Discover/Database/config';

import { ResourceKind } from '../Shared';

interface DatabaseWrapperProps {
  children: React.ReactNode;
}

export function DatabaseWrapper(props: DatabaseWrapperProps) {
  return (
    <JoinTokenProvider timeout={SCRIPT_TIMEOUT}>
      <PingTeleportProvider
        timeout={PING_TIMEOUT}
        interval={PING_INTERVAL}
        resourceKind={ResourceKind.Database}
      >
        {props.children}
      </PingTeleportProvider>
    </JoinTokenProvider>
  );
}
