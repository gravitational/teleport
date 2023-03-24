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

import { sortLogins } from 'teleport/Nodes/useNodes';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

import * as stores from './../stores';
import { useConsoleContext } from './../consoleContextProvider';

import type { Node } from 'teleport/services/nodes';

export default function useNodes({ clusterId, id }: stores.DocumentNodes) {
  const consoleCtx = useConsoleContext();

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'hostname',
    dir: 'ASC',
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
    fetchedData,
    createSshSession,
    changeCluster,
    getNodeSshLogins,
    params,
    ...filteringProps,
    ...paginationProps,
  };
}
