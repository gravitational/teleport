/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useEffect } from 'react';

import Ctx from 'teleport/teleportContext';
import useStickerClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';
import { openNewTab } from 'teleport/lib/util';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

import type { Node } from 'teleport/services/nodes';

export function useNodes(ctx: Ctx) {
  const { isLeafCluster, clusterId } = useStickerClusterId();
  const canCreate = ctx.storeUser.getTokenAccess().create;

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'hostname',
    dir: 'ASC',
  });

  const { fetch, fetchedData, ...paginationProps } = useServerSidePagination({
    fetchFunc: ctx.nodeService.fetchNodes,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  function getNodeLoginOptions(serverId: string) {
    const node = fetchedData.agents.find(node => node.id == serverId);
    return makeOptions(clusterId, node);
  }

  const startSshSession = (login: string, serverId: string) => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId,
      login,
    });

    openNewTab(url);
  };

  return {
    fetchedData,
    canCreate,
    getNodeLoginOptions,
    startSshSession,
    isLeafCluster,
    clusterId,
    params,
    ...filteringProps,
    ...paginationProps,
  };
}

function makeOptions(clusterId: string, node: Node | undefined) {
  const nodeLogins = node?.sshLogins || [];
  const logins = sortLogins(nodeLogins);

  return logins.map(login => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId: node?.id || '',
      login,
    });

    return {
      login,
      url,
    };
  });
}

// sort logins by making 'root' as the first in the list
export const sortLogins = (logins: string[]) => {
  const noRoot = logins.filter(l => l !== 'root').sort();
  if (noRoot.length === logins.length) {
    return logins;
  }
  return ['root', ...noRoot];
};

export type State = ReturnType<typeof useNodes>;
