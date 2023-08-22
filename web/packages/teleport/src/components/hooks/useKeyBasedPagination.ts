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

import { useState, useEffect, useRef } from 'react';
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
 *
 * This hook is intended to be used in tandem with the `useInfiniteScroll` hook.
 * See its documentation for an example.
 */
export function useKeyBasedPagination<T>({
  fetchFunc,
  clusterId,
  filter,
  initialFetchSize = 30,
  fetchMoreSize = 20,
}: Props<T>): State<T> {
  const abortController = useRef<AbortController | null>(null);
  const { attempt, setAttempt } = useAttempt();
  const [finished, setFinished] = useState(false);
  const [resources, setResources] = useState<T[]>([]);
  const [startKey, setStartKey] = useState(null);

  useEffect(() => {
    abortController.current?.abort();
    abortController.current = null;

    setAttempt({ status: '', statusText: '' });
    setFinished(false);
    setResources([]);
    setStartKey(null);
  }, [clusterId, ...resourceFilterToHookDeps(filter)]);

  // This is an additional countermeasure to prevent fetching if `fetch` is
  // called multiple times per React state reconciliation cycle.
  let fetchCalled = false;
  const fetch = async (force: boolean) => {
    if (
      finished ||
      fetchCalled ||
      (!force &&
        (attempt.status === 'processing' || attempt.status === 'failed'))
    ) {
      return;
    }

    try {
      fetchCalled = true;
      setAttempt({ status: 'processing' });
      const limit = resources.length > 0 ? fetchMoreSize : initialFetchSize;
      abortController.current?.abort();
      abortController.current = new AbortController();
      const res = await fetchFunc(
        clusterId,
        {
          ...filter,
          limit,
          startKey,
        },
        abortController.current.signal
      );
      abortController.current = null;
      setResources([...resources, ...res.agents]);
      setStartKey(res.startKey);
      if (!res.startKey) {
        setFinished(true);
      }
      setAttempt({ status: 'success' });
    } catch (err) {
      // Aborting is not really an error here.
      if (isAbortError(err)) {
        setAttempt({ status: '', statusText: '' });
        return;
      }
      setAttempt({ status: 'failed', statusText: err.message });
    }
  };

  return {
    fetch: () => fetch(false),
    forceFetch: () => fetch(true),
    attempt,
    resources,
    finished,
  };
}

const isAbortError = (err: any): boolean =>
  (err instanceof DOMException && err.name === 'AbortError') ||
  (err.cause && isAbortError(err.cause));

export type Props<T> = {
  fetchFunc: (
    clusterId: string,
    params: UrlResourcesParams,
    signal?: AbortSignal
  ) => Promise<ResourcesResponse<T>>;
  clusterId: string;
  filter: ResourceFilter;
  initialFetchSize?: number;
  fetchMoreSize?: number;
};

export type State<T> = {
  /**
   * Attempts to fetch a new batch of data, unless one is already being fetched,
   * or the previous fetch resulted with an error. It is intended to be called
   * as a mere suggestion to fetch more data and can be called multiple times,
   * for example when the user scrolls to the bottom of the page. This is the
   * function that you should pass to `useInfiniteScroll` hook.
   */
  fetch: () => Promise<void>;

  /**
   * Fetches a new batch of data. Cancels a pending request, if there is one.
   * Disregards whether error has previously occurred. Intended for using as an
   * explicit user's action. Don't call it from `useInfiniteScroll`, or you'll
   * risk flooding the server with requests!
   */
  forceFetch: () => Promise<void>;

  attempt: Attempt;
  resources: T[];
  finished: boolean;
};
