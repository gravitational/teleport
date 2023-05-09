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
import { FetchStatus, Page } from 'design/DataTable/types';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import {
  AgentResponse,
  AgentKind,
  AgentFilter,
} from 'teleport/services/agents';
import { UrlResourcesParams } from 'teleport/config';

export function useServerSidePagination<T extends AgentKind>({
  fetchFunc,
  clusterId,
  params,
  pageSize = 15,
}: Props<T>): State<T> {
  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [page, setPage] = useState<Page>({ keys: [], index: 0 });

  const [fetchedData, setFetchedData] = useState<AgentResponse<T>>({
    agents: [],
    startKey: '',
    totalCount: 0,
  });

  let from = 0;
  let to = 0;
  let totalCount = 0;
  if (fetchedData.totalCount) {
    from = page.index * pageSize + 1;
    to = from + fetchedData.agents.length - 1;
    totalCount = fetchedData.totalCount;
  }

  function fetch() {
    setFetchStatus('loading');
    setAttempt({ status: 'processing' });
    fetchFunc(clusterId, { ...params, limit: pageSize })
      .then(res => {
        setFetchedData({
          ...fetchedData,
          agents: res.agents,
          startKey: res.startKey,
          totalCount: res.totalCount,
        });
        setPage({
          keys: ['', res.startKey],
          index: 0,
        });
        setFetchStatus(res.startKey ? '' : 'disabled');
        setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchedData({ ...fetchedData, agents: [], totalCount: 0 });
        setFetchStatus('');
      });
  }

  const fetchNext = () => {
    setFetchStatus('loading');
    fetchFunc(clusterId, {
      ...params,
      limit: pageSize,
      startKey: page.keys[page.index + 1],
    })
      .then(res => {
        setFetchedData({
          ...fetchedData,
          agents: res.agents,
          startKey: res.startKey,
        });
        setPage({
          keys: [...page.keys, res.startKey],
          index: page.index + 1,
        });
        setFetchStatus(res.startKey ? '' : 'disabled');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchStatus('');
      });
  };

  const fetchPrev = () => {
    setFetchStatus('loading');
    fetchFunc(clusterId, {
      ...params,
      limit: pageSize,
      startKey: page.keys[page.index - 1],
    })
      .then(res => {
        setFetchedData({
          ...fetchedData,
          agents: res.agents,
          startKey: res.startKey,
        });
        setPage({
          keys: page.keys.slice(0, -1),
          index: page.index - 1,
        });
        setFetchStatus('');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchStatus('');
      });
  };

  return {
    pageIndicators: { from, to, totalCount } as PageIndicators,
    fetch,
    fetchNext: page.keys[page.index + 1] ? fetchNext : null,
    fetchPrev: page.index > 0 ? fetchPrev : null,
    attempt,
    fetchStatus,
    page,
    pageSize,
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
  pageSize?: number;
};

type State<T extends AgentKind> = {
  pageIndicators: PageIndicators;
  fetch: () => void;
  fetchNext: (() => void) | null;
  fetchPrev: (() => void) | null;
  attempt: Attempt;
  fetchStatus: FetchStatus;
  page: Page;
  pageSize: number;
  fetchedData: AgentResponse<T>;
};

/** Contains the values needed to display 'Showing X - X of X' on the top right of the table. */
export type PageIndicators = {
  /** The position of the first item on the page relative to all items. */
  from: number;
  /** The position of the last item on the page relative to all items. */
  to: number;
  /** The total number of all items. */
  totalCount: number;
};
