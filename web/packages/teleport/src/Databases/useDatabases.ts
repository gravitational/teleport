/*
Copyright 2021-2022 Gravitational, Inc.

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

import Ctx from 'teleport/teleportContext';
import getResourceUrlQueryParams, {
  ResourceUrlQueryParams,
} from 'teleport/getUrlQueryParams';
import useStickyClusterId from 'teleport/useStickyClusterId';
import history from 'teleport/services/history';
import { DatabasesResponse } from 'teleport/services/databases';
import labelClick from 'teleport/labelClick';
import { AgentLabel } from 'teleport/services/agents';

export default function useDatabases(ctx: Ctx) {
  const { search, pathname } = useLocation();
  const [startKeys, setStartKeys] = useState<string[]>([]);
  const { attempt, setAttempt } = useAttempt('processing');
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const username = ctx.storeUser.state.username;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [params, setParams] = useState<ResourceUrlQueryParams>({
    sort: { fieldName: 'name', dir: 'ASC' },
    ...getResourceUrlQueryParams(search),
  });

  const isSearchEmpty = !params?.query && !params?.search;

  const [results, setResults] = useState<DatabasesResponse>({
    databases: [],
    startKey: '',
    totalCount: 0,
  });

  const pageSize = 15;

  const from =
    results.totalCount > 0 ? (startKeys.length - 2) * pageSize + 1 : 0;
  const to = results.totalCount > 0 ? from + results.databases.length - 1 : 0;

  useEffect(() => {
    fetchDatabases();
  }, [clusterId, search]);

  function setSort(sort: SortType) {
    setParams({ ...params, sort });
  }

  function replaceHistory(path: string) {
    history.replace(path);
  }

  function fetchDatabases() {
    setAttempt({ status: 'processing' });
    ctx.databaseService
      .fetchDatabases(clusterId, { ...params, limit: pageSize })
      .then(res => {
        setResults({
          databases: res.agents,
          startKey: res.startKey,
          totalCount: res.totalCount,
        });
        setFetchStatus(res.startKey ? '' : 'disabled');
        setStartKeys(['', res.startKey]);
        setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setResults({ ...results, databases: [], totalCount: 0 });
        setStartKeys(['']);
      });
  }

  const fetchNext = () => {
    setFetchStatus('loading');
    ctx.databaseService
      .fetchDatabases(clusterId, {
        ...params,
        limit: pageSize,
        startKey: results.startKey,
      })
      .then(res => {
        setResults({
          ...results,
          databases: res.agents,
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
    ctx.databaseService
      .fetchDatabases(clusterId, {
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
          databases: res.agents,
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
    attempt,
    canCreate,
    isLeafCluster,
    username,
    clusterId,
    authType,
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
    accessRequestId,
  };
}

export type State = ReturnType<typeof useDatabases>;
