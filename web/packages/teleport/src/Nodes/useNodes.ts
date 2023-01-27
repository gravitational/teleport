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

import { useState, useEffect } from 'react';
import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import { AgentResponse } from 'teleport/services/agents';
import Ctx from 'teleport/teleportContext';
import { StickyCluster } from 'teleport/types';
import cfg from 'teleport/config';
import { openNewTab } from 'teleport/lib/util';
import {
  useUrlFiltering,
  useServerSidePagination,
} from 'teleport/components/hooks';

import type { Node } from 'teleport/services/nodes';

export default function useNodes(ctx: Ctx, stickyCluster: StickyCluster) {
  const { isLeafCluster, clusterId } = stickyCluster;
  const { attempt, setAttempt } = useAttempt('processing');
  const canCreate = ctx.storeUser.getTokenAccess().create;
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
      fetchFunc: ctx.nodeService.fetchNodes,
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

  function getNodeLoginOptions(serverId: string) {
    const node = results.agents.find(node => node.id == serverId);
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

  function fetchNodes() {
    setAttempt({ status: 'processing' });
    ctx.nodeService
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

  return {
    canCreate,
    attempt,
    getNodeLoginOptions,
    startSshSession,
    isLeafCluster,
    clusterId,
    results,
    fetchStatus,
    params,
    pageSize,
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
