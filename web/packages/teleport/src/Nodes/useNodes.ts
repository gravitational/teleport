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
import { useLocation } from 'react-router';
import { FetchStatus, SortType } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import history from 'teleport/services/history';
import Ctx from 'teleport/teleportContext';
import { StickyCluster } from 'teleport/types';
import cfg from 'teleport/config';

import { openNewTab } from 'teleport/lib/util';
import getResourceUrlQueryParams, {
  ResourceUrlQueryParams,
} from 'teleport/getUrlQueryParams';
import labelClick from 'teleport/labelClick';
import { AgentLabel } from 'teleport/services/agents';

import type { Node, NodesResponse } from 'teleport/services/nodes';

export default function useNodes(ctx: Ctx, stickyCluster: StickyCluster) {
  const { isLeafCluster, clusterId } = stickyCluster;
  const { search, pathname } = useLocation();
  const [startKeys, setStartKeys] = useState<string[]>([]);
  const { attempt, setAttempt } = useAttempt('processing');
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [params, setParams] = useState<ResourceUrlQueryParams>({
    sort: { fieldName: 'hostname', dir: 'ASC' },
    ...getResourceUrlQueryParams(search),
  });

  const isSearchEmpty = !params?.query && !params?.search;

  const [results, setResults] = useState<NodesResponse>({
    nodes: [],
    startKey: '',
    totalCount: 0,
  });

  const pageSize = 15;

  const from =
    results.totalCount > 0 ? (startKeys.length - 2) * pageSize + 1 : 0;
  const to = results.totalCount > 0 ? from + results.nodes.length - 1 : 0;

  useEffect(() => {
    fetchNodes();
  }, [clusterId, search]);

  function replaceHistory(path: string) {
    history.replace(path);
  }

  function getNodeLoginOptions(serverId: string) {
    const node = results.nodes.find(node => node.id == serverId);
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

  function setSort(sort: SortType) {
    setParams({ ...params, sort });
  }

  function fetchNodes() {
    setAttempt({ status: 'processing' });
    ctx.nodeService
      .fetchNodes(clusterId, { ...params, limit: pageSize })
      .then(res => {
        setResults({
          nodes: res.agents,
          startKey: res.startKey,
          totalCount: res.totalCount,
        });
        setFetchStatus(res.startKey ? '' : 'disabled');
        setStartKeys(['', res.startKey]);
        setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setResults({ ...results, nodes: [], totalCount: 0 });
        setStartKeys(['']);
      });
  }

  const fetchNext = () => {
    setFetchStatus('loading');
    ctx.nodeService
      .fetchNodes(clusterId, {
        ...params,
        limit: pageSize,
        startKey: results.startKey,
      })
      .then(res => {
        setResults({
          ...results,
          nodes: res.agents,
          startKey: res.startKey,
        });
        setFetchStatus(res.startKey ? '' : 'disabled');
        setStartKeys([...startKeys, res.startKey]);
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  };

  const fetchPrev = () => {
    setFetchStatus('loading');
    ctx.nodeService
      .fetchNodes(clusterId, {
        ...params,
        limit: pageSize,
        startKey: startKeys[startKeys.length - 3],
      })
      .then(res => {
        const tempStartKeys = startKeys;
        tempStartKeys.pop();
        setStartKeys(tempStartKeys);
        setResults({
          ...results,
          nodes: res.agents,
          startKey: res.startKey,
        });
        setFetchStatus('');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  };

  const onLabelClick = (label: AgentLabel) =>
    labelClick(label, params, setParams, pathname, replaceHistory);

  return {
    canCreate,
    attempt,
    getNodeLoginOptions,
    startSshSession,
    isLeafCluster,
    clusterId,
    results,
    fetchNext,
    fetchPrev,
    pageSize,
    from,
    to,
    params,
    setParams,
    startKeys,
    setSort,
    pathname,
    replaceHistory,
    fetchStatus,
    isSearchEmpty,
    onLabelClick,
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
