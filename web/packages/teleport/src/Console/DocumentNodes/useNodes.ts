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

import {
  useServerSidePagination,
  useUrlFiltering,
} from 'teleport/components/hooks';
import type { Node } from 'teleport/services/nodes';

import { useConsoleContext } from './../consoleContextProvider';
import * as stores from './../stores';

export default function useNodes({ clusterId, id }: stores.DocumentNodes) {
  const consoleCtx = useConsoleContext();

  const { params, search, ...filteringProps } = useUrlFiltering({
    sort: {
      fieldName: 'hostname',
      dir: 'ASC',
    },
  });

  const { fetch, fetchedData, ...paginationProps } = useServerSidePagination({
    fetchFunc: consoleCtx.nodesService.fetchNodes,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  function createSshSession(login: string, serverId: string) {
    const url = consoleCtx.getSshDocumentUrl({
      serverId,
      login,
      clusterId,
    });
    consoleCtx.gotoTab({ url });
    consoleCtx.removeDocument(id);
  }

  function changeCluster(value: string) {
    const clusterId = value;
    const url = consoleCtx.getNodeDocumentUrl(clusterId);
    consoleCtx.storeDocs.update(id, {
      url,
      clusterId,
    });

    consoleCtx.gotoTab({ url });
  }

  function getNodeSshLogins(serverId: string) {
    const node = fetchedData.agents.find(node => node.id == serverId);
    return makeOptions(clusterId, node);
  }

  function makeOptions(clusterId: string, node: Node | undefined) {
    const nodeLogins = node?.sshLogins || [];
    const logins = sortLogins(nodeLogins);

    return logins.map(login => {
      const url = consoleCtx.getSshDocumentUrl({
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
  return {
    consoleCtx,
    fetchedData,
    createSshSession,
    changeCluster,
    getNodeSshLogins,
    params,
    ...filteringProps,
    ...paginationProps,
  };
}

// sort logins by making 'root' as the first in the list
export const sortLogins = (logins: string[]) => {
  const noRoot = logins.filter(l => l !== 'root');
  noRoot.sort();
  if (noRoot.length === logins.length) {
    return logins;
  }
  return ['root', ...noRoot];
};
