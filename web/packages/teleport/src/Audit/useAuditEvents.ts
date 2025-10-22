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

import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useHistory, useLocation } from 'react-router';

import type { SortDir, SortType } from 'design/DataTable/types';

import {
  EventRange,
  getRangeOptions,
} from 'teleport/components/EventRangePicker';
import { EventCode, formatters } from 'teleport/services/audit';
import Ctx from 'teleport/teleportContext';

const PAGE_SIZE = 50;

export default function useAuditEvents(
  ctx: Ctx,
  clusterId: string,
  eventCode?: EventCode
) {
  const history = useHistory();
  const location = useLocation();

  const rangeOptions = useMemo(() => getRangeOptions(), []);
  const [currentPageIndex, setCurrentPageIndex] = useState(0);

  const queryParams = useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );

  const fromParam = queryParams.get('from');
  const toParam = queryParams.get('to');
  const search = queryParams.get('search') || '';
  const orderParam = queryParams.get('order');
  const sortDir: SortDir = orderParam?.toUpperCase() === 'ASC' ? 'ASC' : 'DESC';

  let range = rangeOptions[0];

  if (fromParam && toParam) {
    range = {
      from: new Date(fromParam),
      to: new Date(toParam),
      isCustom: true,
    };
  }

  const filterBy = eventCode ? formatters[eventCode].type : '';

  const {
    data,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    isSuccess,
  } = useInfiniteQuery({
    queryKey: [
      'audit_events',
      clusterId,
      range.from.toISOString(),
      range.to.toISOString(),
      filterBy,
      search,
      sortDir,
    ],
    queryFn: ({ pageParam }) =>
      ctx.auditService.fetchEventsV2(clusterId, {
        ...range,
        filterBy,
        startKey: pageParam,
        limit: PAGE_SIZE,
        search,
        order: sortDir,
      }),
    initialPageParam: '',
    getNextPageParam: lastPage => lastPage.startKey || undefined,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  const currentPageEvents = useMemo(() => {
    if (!data || data.pages.length === 0) {
      return [];
    }

    const safeIndex = Math.min(currentPageIndex, data.pages.length - 1);
    if (safeIndex < 0) {
      return [];
    }

    return data.pages[safeIndex]?.events || [];
  }, [data, currentPageIndex]);

  const onFetchNext = useCallback(() => {
    if (data && currentPageIndex < data.pages.length - 1) {
      setCurrentPageIndex(prev => prev + 1);
      return;
    }
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage().then(() => {
        setCurrentPageIndex(prev => prev + 1);
      });
    }
  }, [currentPageIndex, data, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const onFetchPrev = useCallback(() => {
    if (currentPageIndex > 0) {
      setCurrentPageIndex(prev => prev - 1);
    }
  }, [currentPageIndex]);

  const setRange = useCallback(
    (newRange: EventRange) => {
      setCurrentPageIndex(0);

      const params = new URLSearchParams(location.search);
      params.set('from', newRange.from.toISOString());
      params.set('to', newRange.to.toISOString());

      history.replace({
        pathname: location.pathname,
        search: params.toString(),
      });
    },
    [history, location]
  );

  const setSearch = useCallback(
    (newSearch: string) => {
      setCurrentPageIndex(0);

      const params = new URLSearchParams(location.search);
      if (newSearch) {
        params.set('search', newSearch);
      } else {
        params.delete('search');
      }

      history.replace({
        pathname: location.pathname,
        search: params.toString(),
      });
    },
    [history, location]
  );

  const setSort = useCallback(
    (nextSort: SortType) => {
      setCurrentPageIndex(0);

      const params = new URLSearchParams(location.search);
      const nextDir: SortDir = nextSort.dir === 'ASC' ? 'ASC' : 'DESC';
      params.set('order', nextDir);

      history.replace({
        pathname: location.pathname,
        search: params.toString(),
      });
    },
    [history, location]
  );

  useEffect(() => {
    setCurrentPageIndex(0);
  }, [location.search]);

  const sort: SortType = { fieldName: 'time', dir: sortDir };

  return {
    events: currentPageEvents,
    onFetchNext:
      hasNextPage || currentPageIndex < (data?.pages.length || 0) - 1
        ? onFetchNext
        : undefined,
    onFetchPrev: currentPageIndex > 0 ? onFetchPrev : undefined,
    isLoadingPage: isLoading || isFetchingNextPage,
    error,
    isSuccess,
    clusterId,
    range,
    setRange,
    rangeOptions,
    search,
    setSearch,
    sort,
    setSort,
    ctx,
  };
}

export type State = ReturnType<typeof useAuditEvents>;
