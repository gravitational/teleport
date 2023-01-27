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

import { useEffect, useState } from 'react';
import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import { AgentResponse } from 'teleport/services/agents';
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
  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [results, setResults] = useState<AgentResponse<Node>>({
    agents: [],
    startKey: '',
    totalCount: 0,
  });

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'hostname',
    dir: 'ASC',
  });

  const { setStartKeys, pageSize, ...paginationProps } =
    useServerSidePagination({
      fetchFunc: consoleCtx.fetchNodes,
      clusterId,
      params,
      results,
      setResults,
      setFetchStatus,
      setAttempt,
    });

  useEffect(() => {
    fetchNodes();
  }, [clusterId, search]);

  function fetchNodes() {
    setAttempt({ status: 'processing' });
    consoleCtx
      .fetchNodes(clusterId, { ...params, limit: pageSize })
      .then(res => {
        setResults(res);
        setFetchStatus(res.startKey ? '' : 'disabled');
        setStartKeys(['', res.startKey]);
        setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setResults({ ...results, agents: [], totalCount: 0 });
        setStartKeys(['']);
      });
  }

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
    const node = results.agents.find(node => node.id == serverId);
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
    attempt,
    createSshSession,
    changeCluster,
    getNodeSshLogins,
    results,
    fetchStatus,
    pageSize,
    params,
    ...filteringProps,
    ...paginationProps,
  };
}
