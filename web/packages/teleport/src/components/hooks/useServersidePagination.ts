/*
Copyright 2022 Gravitational, Inc.

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

import { useState } from 'react';
import { FetchStatus } from 'design/DataTable/types';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { AgentResponse, AgentKind } from 'teleport/services/agents';
import { UrlResourcesParams } from 'teleport/config';
import { ResourceUrlQueryParams } from './useUrlFiltering/useUrlFiltering';

export default function useServerSidePagination<T extends AgentKind>({
  fetchFunc,
  clusterId,
  params,
  results,
  setResults,
  setFetchStatus,
  setAttempt,
  pageSize = 15,
}: PaginationArgs<T>) {
  const [startKeys, setStartKeys] = useState<string[]>([]);

  const from =
    results.totalCount > 0 ? (startKeys.length - 2) * pageSize + 1 : 0;
  const to = results.totalCount > 0 ? from + results.agents.length - 1 : 0;

  const fetchNext = () => {
    setFetchStatus('loading');
    fetchFunc(clusterId, {
      ...params,
      limit: pageSize,
      startKey: results.startKey,
    })
      .then(res => {
        setResults({
          ...results,
          agents: res.agents,
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
    fetchFunc(clusterId, {
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
          agents: res.agents,
          startKey: res.startKey,
        });
        setFetchStatus('');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  };

  return { from, to, fetchNext, fetchPrev, startKeys, setStartKeys, pageSize };
}

type PaginationArgs<T extends AgentKind> = {
  fetchFunc: (
    clusterId: string,
    params: UrlResourcesParams
  ) => Promise<AgentResponse<T>>;
  clusterId: string;
  params: ResourceUrlQueryParams;
  results: AgentResponse<T>;
  setResults: (results: AgentResponse<T>) => void;
  setFetchStatus: (fetchStatus: FetchStatus) => void;
  setAttempt: (attempt: Attempt) => void;
  pageSize?: number;
};
