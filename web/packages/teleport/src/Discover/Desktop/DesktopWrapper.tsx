import React, { useEffect } from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { PING_INTERVAL } from 'teleport/Discover/Desktop/config';

import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';

import { ResourceKind } from '../Shared';

interface DesktopWrapperProps {
  children: React.ReactNode;
}

export function DesktopWrapper(props: DesktopWrapperProps) {
  useEffect(() => {
    return () => {
      // once the user leaves the desktop setup flow, delete the existing token
      clearCachedJoinTokenResult(ResourceKind.Desktop);
    };
  }, []);

  return (
    <PingTeleportProvider
      interval={PING_INTERVAL}
      resourceKind={ResourceKind.Desktop}
    >
      {props.children}
    </PingTeleportProvider>
  );
}
