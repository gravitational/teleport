import React, { useEffect } from 'react';

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';

import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';

import { ResourceKind } from '../Shared';

const PING_INTERVAL = 1000 * 3; // 3 seconds

export function KubeWrapper(props: WrapperProps) {
  useEffect(() => {
    return () => {
      // once the user leaves this flow, delete the existing token
      clearCachedJoinTokenResult(ResourceKind.Kubernetes);
    };
  }, []);

  return (
    <PingTeleportProvider
      interval={PING_INTERVAL}
      resourceKind={ResourceKind.Kubernetes}
    >
      {props.children}
    </PingTeleportProvider>
  );
}

interface WrapperProps {
  children: React.ReactNode;
}
