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

import { useState, useRef, useCallback } from 'react';

import { ResourcesResponse } from 'teleport/services/agents';
import { ApiError } from 'teleport/services/api/parseError';

import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';

/**
 * Supports fetching more data from the server when more data is available. Pass
 * a `fetchFunc` that retrieves a single batch of data. After the initial
 * request, the server is expected to return a `startKey` field that denotes the
 * next `startKey` to use for the next request.
 *
 * The hook maintains an invariant that there's only up to one valid
 * pending request at all times. Any out-of-order responses are discarded.
 */
export function useKeyBasedPagination<T>({
  fetchFunc,
  initialFetchSize = 30,
  fetchMoreSize = 20,
}: KeyBasedPaginationOptions<T>): KeyBasedPagination<T> {
  const { attempt, setAttempt } = useAttempt();
  const [finished, setFinished] = useState(false);
  const [resources, setResources] = useState<T[]>([]);
  const [startKey, setStartKey] = useState<string | null>(null);

  // Ephemeral state used solely to coordinate fetch calls, doesn't need to
  // cause rerenders.
  const abortController = useRef<AbortController | null>(null);
  const pendingPromise = useRef<Promise<ResourcesResponse<T>> | null>(null);

  const clear = useCallback(() => {
    abortController.current?.abort();
    abortController.current = null;
    pendingPromise.current = null;

    setAttempt({ status: '', statusText: '' });
    setFinished(false);
    setResources([]);
    setStartKey(null);
  }, [setAttempt]);

  const fetchInternal = async (force: boolean) => {
    if (
      finished ||
      (!force &&
        (pendingPromise.current ||
          attempt.status === 'processing' ||
          attempt.status === 'failed'))
    ) {
      return;
    }

    try {
      setAttempt({ status: 'processing' });
      abortController.current?.abort();
      abortController.current = new AbortController();
      const limit = resources.length > 0 ? fetchMoreSize : initialFetchSize;
      const newPromise = fetchFunc(
        {
          limit,
          startKey,
        },
        abortController.current.signal
      );
      pendingPromise.current = newPromise;

      const res = await newPromise;

      if (pendingPromise.current !== newPromise) {
        return;
      }

      pendingPromise.current = null;
      abortController.current = null;
      // Note: even though the old resources appear in this call, this _is_ more
      // correct than a standard practice of using a callback form of
      // `setState`. This is because, contrary to an "increasing a counter"
      // analogy, adding given set of resources to the current set of resources
      // strictly depends on the exact set of resources that were there when
      // `fetch` was called. This shouldn't make a difference in practice (we
      // have other ways to mitigate discrepancies here), but better safe than
      // sorry.
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
      let statusCode;
      if (err instanceof ApiError && err.response) {
        statusCode = err.response.status;
      }
      setAttempt({ status: 'failed', statusText: err.message, statusCode });
    }
  };

  const fetch = useCallback(
    (options: { force?: boolean }) => fetchInternal(!!options?.force),
    [fetchFunc, startKey, resources, finished, attempt]
  );

  return {
    fetch,
    clear,
    attempt,
    resources,
    finished,
  };
}

export type KeyBasedPaginationOptions<T> = {
  fetchFunc: (
    paginationParams: { limit: number; startKey: string },
    signal?: AbortSignal
  ) => Promise<ResourcesResponse<T>>;
  initialFetchSize?: number;
  fetchMoreSize?: number;
};

type KeyBasedPagination<T> = {
  /**
   * Attempts to fetch a new batch of data, unless one is already being fetched,
   * or the previous fetch resulted with an error. It is intended to be called
   * as a mere suggestion to fetch more data and can be called multiple times,
   * for example, when the user scrolls to the bottom of the page.
   *
   * @param options.force Cancels a pending request, if there is one.
   * Disregards whether error has previously occurred. Intended for using as an
   * explicit user's action. Don't call it from `useInfiniteScroll`, or you'll
   * risk flooding the server with requests!
   */
  fetch(options?: { force?: boolean }): Promise<void>;
  /** Aborts a pending request and clears the state. **/
  clear(): void;
  attempt: Attempt;
  resources: T[];
  finished: boolean;
};
