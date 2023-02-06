import React, { useCallback, useContext, useEffect, useState } from 'react';

import { useTeleport } from 'teleport';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import {
  INTERNAL_RESOURCE_ID_LABEL_KEY,
  JoinToken,
} from 'teleport/services/joinToken';
import { ResourceKind } from 'teleport/Discover/Shared/ResourceKind';

interface PingTeleportContextState<T> {
  active: boolean;
  start: (joinToken: JoinToken, alternateSearchTerm?: string) => void;
  result: T | null;
}

const pingTeleportContext =
  React.createContext<PingTeleportContextState<any>>(null);

export function PingTeleportProvider<T>(props: {
  interval?: number;
  children?: React.ReactNode;
  resourceKind: ResourceKind;
}) {
  const ctx = useTeleport();

  // Start in an inactive state so that polling doesn't begin
  // until a call to usePingTeleport passes in the joinToken and optional
  // alternateSearchTerm.
  const [active, setActive] = useState(false);

  // alternateSearchTerm, when set, will be used as the search term
  // instead of the default search term which is the internal resource ID.
  // Only applies to certain resource's eg: looking up database server
  // that proxies a database that goes by this alternateSearchTerm (eg. resourceName).
  const [alternateSearchTerm, setAlternateSearchTerm] = useState('');

  // joinToken is passed in by the caller of usePingTeleport, which is necessary so
  // that useJoinTokenSuspender can be called and used with <Suspense>.
  const [joinToken, setJoinToken] = useState<JoinToken | null>(null);

  const result = usePoll<T>(
    signal =>
      servicesFetchFn(signal).then(res => {
        if (res.agents.length) {
          return res.agents[0];
        }

        return null;
      }),
    active,
    props.interval
  );

  function servicesFetchFn(signal: AbortSignal) {
    const clusterId = ctx.storeUser.getClusterId();
    let search = `${INTERNAL_RESOURCE_ID_LABEL_KEY} ${joinToken.internalResourceId}`;
    if (alternateSearchTerm) {
      search = alternateSearchTerm;
    }
    const request = {
      search,
      limit: 1,
    };
    switch (props.resourceKind) {
      case ResourceKind.Server:
        return ctx.nodeService.fetchNodes(clusterId, request, signal);
      case ResourceKind.Desktop:
        return ctx.desktopService.fetchDesktopServices(
          clusterId,
          request,
          signal
        );
      case ResourceKind.Kubernetes:
        return ctx.kubeService.fetchKubernetes(clusterId, request, signal);
      case ResourceKind.Database:
        return ctx.databaseService.fetchDatabases(clusterId, request, signal);
    }
  }

  // start is called by usePingTeleport. It begins polling if polling is not
  // yet active AND we haven't yet found a result.
  const start = useCallback(
    (joinToken: JoinToken, alternateSearchTerm?: string) => {
      if (!active && !result) {
        setJoinToken(joinToken);
        setAlternateSearchTerm(alternateSearchTerm);
        setActive(true);
      }
    },
    [active, result]
  );

  useEffect(() => {
    if (result) {
      // Once we get a result, stop the polling.
      // This result will be passed down to all consumers of
      // this context.
      setActive(false);
    }
  }, [result]);

  return (
    <pingTeleportContext.Provider
      value={{
        active,
        start,
        result,
      }}
    >
      {props.children}
    </pingTeleportContext.Provider>
  );
}

export function usePingTeleport<T>(
  joinToken: JoinToken,
  alternateSearchTerm?: string
) {
  const ctx = useContext<PingTeleportContextState<T>>(pingTeleportContext);

  useEffect(() => {
    ctx.start(joinToken, alternateSearchTerm);
  }, []);

  return ctx;
}
