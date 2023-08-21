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

import { useState, useEffect } from 'react';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import {
  ResourcesResponse,
  ResourceFilter,
  resourceFilterToHookDeps,
} from 'teleport/services/agents';
import { UrlResourcesParams } from 'teleport/config';

/**
 * Supports fetching more data from the server when more data is available. Pass
 * a `fetchFunc` that retrieves a single batch of data. After the initial
 * request, the server is expected to return a `startKey` field that denotes the
 * next `startKey` to use for the next request.
 */
export function useKeyBasedPagination<T>({
  fetchFunc,
  clusterId,
  filter,
  initialFetchSize = 30,
  fetchMoreSize = 20,
}: Props<T>): State<T> {
  const { attempt, setAttempt } = useAttempt();
  const [finished, setFinished] = useState(false);

  const [fetchedData, setFetchedData] = useState<ResourcesResponse<T>>({
    agents: [],
    startKey: '',
    totalCount: 0,
  });

  useEffect(() => {
    setAttempt({ status: '', statusText: '' });
    setFinished(false);
    setFetchedData({ agents: [], startKey: '', totalCount: 0 });
  }, [clusterId, ...resourceFilterToHookDeps(filter)]);

  const fetch = async () => {
    if (
      finished ||
      attempt.status === 'processing' ||
      attempt.status === 'failed'
    ) {
      return;
    }
    try {
      setAttempt({ status: 'processing' });
      const limit =
        fetchedData.agents.length > 0 ? fetchMoreSize : initialFetchSize;
      const res = await fetchFunc(clusterId, {
        ...filter,
        limit,
        startKey: fetchedData.startKey,
      });
      const { startKey, agents } = res;
      setFetchedData({
        ...fetchedData,
        agents: [...fetchedData.agents, ...agents],
        startKey,
      });
      if (!startKey) {
        setFinished(true);
      }
      setAttempt({ status: 'success' });
    } catch (err) {
      setAttempt({ status: 'failed', statusText: err.message });
    }
  };

  return {
    fetch,
    attempt,
    fetchedData,
  };
}

type Props<T> = {
  fetchFunc: (
    clusterId: string,
    params: UrlResourcesParams
  ) => Promise<ResourcesResponse<T>>;
  clusterId: string;
  filter: ResourceFilter;
  initialFetchSize?: number;
  fetchMoreSize?: number;
};

export type State<T> = {
  fetch: (() => void) | null;
  attempt: Attempt;
  fetchedData: ResourcesResponse<T>;
};
