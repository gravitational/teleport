/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
  start: (tokenOrTerm: JoinToken | string) => void;
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
  // until a call to usePingTeleport passes in the joinToken or
  // searchTerm.
  const [active, setActive] = useState(false);

  // searchTerm can be passed in by the caller of usePingTeleport
  // to be used as the search term.
  // Applies to certain resource's eg: looking up database server
  // that proxies a database that goes by this searchTerm (eg. resourceName).
  const [searchTerm, setSearchTerm] = useState('');

  // joinToken can be passed in by the caller of usePingTeleport to have its
  // internalResourceId used as the search term.
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
    const search =
      searchTerm ||
      `${INTERNAL_RESOURCE_ID_LABEL_KEY} ${joinToken.internalResourceId}`;
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
  // start updates state to start polling.
  const start = useCallback((tokenOrTerm: JoinToken | string) => {
    if (typeof tokenOrTerm === 'string') {
      setSearchTerm(tokenOrTerm);
    } else {
      setJoinToken(tokenOrTerm);
    }
    setActive(true);
  }, []);

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

/**
 * usePingTeleport, when first called within a component hierarchy wrapped by
 * PingTeleportProvider, will poll the server for the resource described by
 * the internal resource id on the joinToken or the search term.
 */
export function usePingTeleport<T>(tokenOrTerm: JoinToken | string) {
  const ctx = useContext<PingTeleportContextState<T>>(pingTeleportContext);

  useEffect(() => {
    // start polling only on the first call to usePingTeleport
    if (!ctx.active && !ctx.result) {
      ctx.start(tokenOrTerm);
    }
  }, []);

  return ctx;
}
