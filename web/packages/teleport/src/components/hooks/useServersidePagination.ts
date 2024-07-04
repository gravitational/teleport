/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState, Dispatch, SetStateAction } from 'react';
import { FetchStatus, Page } from 'design/DataTable/types';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import { ResourcesResponse, ResourceFilter } from 'teleport/services/agents';
import { UrlResourcesParams } from 'teleport/config';

export function useServerSidePagination<T>({
  fetchFunc,
  clusterId,
  params,
  pageSize = 15,
}: Props<T>): SeversidePagination<T> {
  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [page, setPage] = useState<Page>({ keys: [], index: 0 });

  const [fetchedData, setFetchedData] = useState<ResourcesResponse<T>>({
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
    modifyFetchedData: setFetchedData,
  };
}

type Props<T> = {
  fetchFunc: (
    clusterId: string,
    params: UrlResourcesParams
  ) => Promise<ResourcesResponse<T>>;
  clusterId: string;
  params: ResourceFilter;
  pageSize?: number;
};

export type SeversidePagination<T> = {
  pageIndicators: PageIndicators;
  fetch: () => void;
  fetchNext: (() => void) | null;
  fetchPrev: (() => void) | null;
  attempt: Attempt;
  fetchStatus: FetchStatus;
  page: Page;
  pageSize: number;
  fetchedData: ResourcesResponse<T>;
  /** Allows modifying the fetched data. */
  modifyFetchedData: Dispatch<SetStateAction<ResourcesResponse<T>>>;
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
