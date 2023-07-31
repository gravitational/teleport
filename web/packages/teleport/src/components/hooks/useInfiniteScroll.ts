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
  UnifiedResource,
  AgentFilter,
} from 'teleport/services/agents';
import { UrlResourcesParams } from 'teleport/config';

/**
 * Supports fetching more data from the server when more data is available. Pass
 * a `fetchFunc` that retrieves a single batch of data. After the initial
 * request, the server is expected to return a `startKey` field that denotes the
 * next `startKey` to use for the next request.
 */
export function useInfiniteScroll<T extends UnifiedResource>({
  fetchFunc,
  clusterId,
  params,
  initialFetchSize = 30,
  fetchMoreSize = 20,
}: Props<T>): State<T> {
  const { attempt, setAttempt } = useAttempt('processing');

  const [fetchedData, setFetchedData] = useState<AgentResponse<T>>({
    agents: [],
    startKey: '',
    totalCount: 0,
  });

  const fetch = async () => {
    setAttempt({ status: 'processing' });
    try {
      const res = await fetchFunc(clusterId, {
        ...params,
        limit: initialFetchSize,
        startKey: '',
      });

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
    if (attempt.status === 'processing' || !fetchedData.startKey) {
      return;
    }
    try {
      setAttempt({ status: 'processing' });
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

type Props<T extends UnifiedResource> = {
  fetchFunc: (
    clusterId: string,
    params: UrlResourcesParams
  ) => Promise<AgentResponse<T>>;
  clusterId: string;
  params: AgentFilter;
  initialFetchSize?: number;
  fetchMoreSize?: number;
};

type State<T extends UnifiedResource> = {
  fetch: (() => void) | null;
  fetchMore: (() => void) | null;
  attempt: Attempt;
  fetchedData: AgentResponse<T>;
};
