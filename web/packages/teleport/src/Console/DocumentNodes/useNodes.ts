/*
Copyright 2019 Gravitational, Inc.

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
import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import history from 'teleport/services/history';
import { NodesResponse } from 'teleport/services/nodes';
import getResourceUrlQueryParams, {
  ResourceUrlQueryParams,
} from 'teleport/getUrlQueryParams';
import { SortType } from 'teleport/components/ServersideSearchPanel';
import { useConsoleContext } from './../consoleContextProvider';
import * as stores from './../stores';

export default function useNodes({ clusterId, id }: stores.DocumentNodes) {
  const consoleCtx = useConsoleContext();
  const { search, pathname } = useLocation();
  const [startKeys, setStartKeys] = useState<string[]>([]);
  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [params, setParams] = useState<ResourceUrlQueryParams>(() =>
    getResourceUrlQueryParams(search)
  );

  const [results, setResults] = useState<NodesResponse & { logins: string[] }>({
    logins: [],
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
      .then(({ logins, nodesRes }) => {
        setResults({ logins, ...nodesRes });
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
      .then(({ logins, nodesRes }) => {
        setResults({
          logins,
          ...results,
          nodes: nodesRes.nodes,
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
      .then(({ logins, nodesRes }) => {
        setResults({
          logins,
          ...results,
          nodes: nodesRes.nodes,
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
    return results.logins.map(login => ({
      login,
      url: consoleCtx.getSshDocumentUrl({ serverId, login, clusterId }),
    }));
  }

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
  };
}
