/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState } from 'react';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import {
  AgentResponse,
  AgentKind,
  AgentFilter,
} from 'teleport/services/agents';
import { UrlResourcesParams } from 'teleport/config';

export function useInfiniteScroll<T extends AgentKind>({
  fetchFunc,
  clusterId,
  params,
  initialFetchSize = 20,
  fetchMoreSize = 24,
}: Props<T>): State<T> {
  const { attempt, setAttempt } = useAttempt('processing');

  const [fetchedData, setFetchedData] = useState<AgentResponse<T>>({
    agents: [],
    startKey: '',
    totalCount: 0,
  });

  const fetch = async () => {
    try {
      const res = await fetchFunc(clusterId, {
        ...params,
        limit: initialFetchSize,
        startKey: '',
      });
      console.log('res', res);

      setFetchedData({
        ...fetchedData,
        agents: res.agents,
        startKey: res.startKey,
        totalCount: res.totalCount,
      });
      setAttempt({ status: 'success' });
    } catch (err) {
      setAttempt({ status: 'failed', statusText: err.message });
      setFetchedData({ ...fetchedData, agents: [], totalCount: 0 });
    }
  };

  const fetchMore = async () => {
    try {
      const res = await fetchFunc(clusterId, {
        ...params,
        limit: fetchMoreSize,
        startKey: fetchedData.startKey,
      });
      setFetchedData({
        ...fetchedData,
        agents: [...fetchedData.agents, ...res.agents],
        startKey: res.startKey,
      });
      setAttempt({ status: 'success' });
    } catch (err) {
      setAttempt({ status: 'failed', statusText: err.message });
    }
  };

  return {
    fetch,
    fetchMore,
    attempt,
    fetchedData,
  };
}

type Props<T extends AgentKind> = {
  fetchFunc: (
    clusterId: string,
    params: UrlResourcesParams
  ) => Promise<AgentResponse<T>>;
  clusterId: string;
  params: AgentFilter;
  initialFetchSize?: number;
  fetchMoreSize?: number;
};

type State<T extends AgentKind> = {
  fetch: (() => void) | null;
  fetchMore: (() => void) | null;
  attempt: Attempt;
  fetchedData: AgentResponse<T>;
};
