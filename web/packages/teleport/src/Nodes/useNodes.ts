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
    sort: {
      fieldName: 'hostname',
      dir: 'ASC',
    },
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
