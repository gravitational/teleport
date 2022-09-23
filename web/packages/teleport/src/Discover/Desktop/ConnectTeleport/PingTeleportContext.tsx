import React, { useCallback, useContext, useEffect, useState } from 'react';

import { useTeleport } from 'teleport';
import { usePoll } from 'teleport/Discover/Desktop/ConnectTeleport/usePoll';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';
import { useJoinTokenValue } from 'teleport/Discover/Desktop/ConnectTeleport/JoinTokenContext';
import { WindowsDesktopService } from 'teleport/services/desktops';

interface PingTeleportContextState {
  active: boolean;
  start: () => void;
  timeout: number;
  timedOut: boolean;
  result: WindowsDesktopService | null;
}

const pingTeleportContext = React.createContext<PingTeleportContextState>(null);

export function PingTeleportProvider(props: {
  timeout: number;
  interval?: number;
  children?: React.ReactNode;
}) {
  const ctx = useTeleport();

  const [active, setActive] = useState(false);
  const [timeout, setPollTimeout] = useState<number>(null);

  const joinToken = useJoinTokenValue();

  const { timedOut, result } = usePoll(
    signal =>
      ctx.desktopService
        .fetchDesktopServices(
          ctx.storeUser.getClusterId(),
          {
            search: `${INTERNAL_RESOURCE_ID_LABEL_KEY} ${joinToken.internalResourceId}`,
            limit: 1,
          },
          signal
        )
        .then(res => {
          if (res.agents.length) {
            return res.agents[0];
          }

          return null;
        }),
    timeout,
    active,
    props.interval
  );

  useEffect(() => {
    if (active && Date.now() > timeout) {
      setActive(false);
    }
  }, [active, timeout, timedOut]);

  const start = useCallback(() => {
    setPollTimeout(Date.now() + props.timeout);
    setActive(true);
  }, [props.timeout]);

  useEffect(() => {
    if (result) {
      setPollTimeout(null);
      setActive(false);
    }
  }, [result]);

  return (
    <pingTeleportContext.Provider
      value={{ active, start, result, timedOut, timeout }}
    >
      {props.children}
    </pingTeleportContext.Provider>
  );
}

export function usePingTeleport() {
  const ctx = useContext(pingTeleportContext);

  useEffect(() => {
    if (!ctx.active) {
      ctx.start();
    }
  }, []);

  return ctx;
}
