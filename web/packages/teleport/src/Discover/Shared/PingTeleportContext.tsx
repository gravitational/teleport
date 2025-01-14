/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { useCallback, useContext, useEffect, useState } from 'react';

import { useTeleport } from 'teleport';
import { ResourceKind } from 'teleport/Discover/Shared/ResourceKind';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import {
  INTERNAL_RESOURCE_ID_LABEL_KEY,
  JoinToken,
} from 'teleport/services/joinToken';

interface PingTeleportContextState<T> {
  active: boolean;
  start: (tokenOrTerm: JoinToken | string) => void;
  result: T | null;
  stop: () => void;
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
        if (res?.agents?.length) {
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
        stop: () => setActive(false),
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

    return () => ctx.stop();
  }, []);

  return ctx;
}
