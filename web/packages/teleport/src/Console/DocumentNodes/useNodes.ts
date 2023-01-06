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
import { useLocation } from 'react-router';
import { FetchStatus, SortType } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import history from 'teleport/services/history';

import getResourceUrlQueryParams, {
  ResourceUrlQueryParams,
} from 'teleport/getUrlQueryParams';

import labelClick from 'teleport/labelClick';
import { AgentLabel } from 'teleport/services/agents';
import { sortLogins } from 'teleport/Nodes/useNodes';

import * as stores from './../stores';
import { useConsoleContext } from './../consoleContextProvider';

import type { Node, NodesResponse } from 'teleport/services/nodes';

export default function useNodes({ clusterId, id }: stores.DocumentNodes) {
  const consoleCtx = useConsoleContext();
  const { search, pathname } = useLocation();
  const [startKeys, setStartKeys] = useState<string[]>([]);
  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [params, setParams] = useState<ResourceUrlQueryParams>({
    sort: { fieldName: 'hostname', dir: 'ASC' },
    ...getResourceUrlQueryParams(search),
  });

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

  function setSort(sort: SortType) {
    setParams({ ...params, sort });
  }

  function fetchNodes() {
    setAttempt({ status: 'processing' });
    consoleCtx
      .fetchNodes(clusterId, { ...params, limit: pageSize })
      .then(({ nodesRes }) => {
        setResults({
          nodes: nodesRes.agents,
          startKey: nodesRes.startKey,
          totalCount: nodesRes.totalCount,
        });
        setFetchStatus(nodesRes.startKey ? '' : 'disabled');
        setStartKeys(['', nodesRes.startKey]);
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
    consoleCtx
      .fetchNodes(clusterId, {
        ...params,
        limit: pageSize,
        startKey: results.startKey,
      })
      .then(({ nodesRes }) => {
        setResults({
          ...results,
          nodes: nodesRes.agents,
          startKey: nodesRes.startKey,
        });
        setFetchStatus(nodesRes.startKey ? '' : 'disabled');
        setStartKeys([...startKeys, nodesRes.startKey]);
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  };

  const fetchPrev = () => {
    setFetchStatus('loading');
    consoleCtx
      .fetchNodes(clusterId, {
        ...params,
        limit: pageSize,
        startKey: startKeys[startKeys.length - 3],
      })
      .then(({ nodesRes }) => {
        setResults({
          ...results,
          nodes: nodesRes.agents,
          startKey: nodesRes.startKey,
        });
        const tempStartKeys = startKeys;
        tempStartKeys.pop();
        setStartKeys(tempStartKeys);
        setFetchStatus(nodesRes.startKey ? '' : 'disabled');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  };

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
    const node = results.nodes.find(node => node.id == serverId);
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

  const onLabelClick = (label: AgentLabel) =>
    labelClick(label, params, setParams, pathname, replaceHistory);

  return {
    attempt,
    createSshSession,
    changeCluster,
    getNodeSshLogins,
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
    onLabelClick,
  };
}
