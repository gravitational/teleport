import React, { useEffect } from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { PING_INTERVAL } from 'teleport/Discover/Database/config';

import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinToken';

import { ResourceKind } from '../Shared';

interface DatabaseWrapperProps {
  children: React.ReactNode;
}

export function DatabaseWrapper(props: DatabaseWrapperProps) {
  useEffect(() => {
    return () => {
      // once the user leaves the desktop setup flow, delete the existing token
      clearCachedJoinTokenResult(ResourceKind.Database);
    };
  }, []);

  return (
    <PingTeleportProvider
      interval={PING_INTERVAL}
      resourceKind={ResourceKind.Database}
    >
      {props.children}
    </PingTeleportProvider>
  );
}
