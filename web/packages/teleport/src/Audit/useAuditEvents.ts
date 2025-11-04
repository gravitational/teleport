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
import { endOfDay, startOfDay } from 'date-fns';
import { useCallback, useMemo } from 'react';
import { useHistory, useLocation } from 'react-router';

import type { SortDir, SortType } from 'design/DataTable/types';

import { EventRange } from 'teleport/components/EventRangePicker';
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

  const queryParams = useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );

  const fromParam = queryParams.get('from');
  const toParam = queryParams.get('to');
  const search = queryParams.get('search') || '';
  const orderParam = queryParams.get('order');
  const sortDir: SortDir = orderParam?.toUpperCase() === 'ASC' ? 'ASC' : 'DESC';

  const filterBy = eventCode ? formatters[eventCode].type : '';

  const {
    data,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    isSuccess,
    refetch,
    isError,
  } = useInfiniteQuery({
    queryKey: [
      'audit_events',
      clusterId,
      fromParam,
      toParam,
      filterBy,
      search,
      sortDir,
    ],
    queryFn: ({ pageParam, signal }) =>
      ctx.auditService.fetchEventsV2(
        clusterId,
        {
          from: fromParam ? startOfDay(new Date(fromParam)) : undefined,
          to: toParam ? endOfDay(new Date(toParam)) : undefined,
          filterBy,
          startKey: pageParam,
          limit: PAGE_SIZE,
          search,
          order: sortDir,
        },
        signal
      ),
    initialPageParam: '',
    getNextPageParam: lastPage => lastPage.startKey || undefined,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  // Flatten all pages into a single array for infinite scroll
  const events = useMemo(() => {
    if (!data || data.pages.length === 0) {
      return [];
    }
    return data.pages.flatMap(page => page.events);
  }, [data]);

  const setRange = useCallback(
    (newRange: EventRange) => {
      const params = new URLSearchParams(location.search);
      params.set('from', newRange.from.toISOString());
      params.set('to', newRange.to.toISOString());

      history.push({
        pathname: location.pathname,
        search: params.toString(),
      });
    },
    [history, location]
  );

  const setSearch = useCallback(
    (newSearch: string) => {
      const params = new URLSearchParams(location.search);
      if (newSearch) {
        params.set('search', newSearch);
      } else {
        params.delete('search');
      }

      history.push({
        pathname: location.pathname,
        search: params.toString(),
      });
    },
    [history, location]
  );

  const setSort = useCallback(
    (nextSort: SortType) => {
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

  const sort: SortType = { fieldName: 'time', dir: sortDir };

  return {
    events,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    error,
    isSuccess,
    refetch,
    isError,
    clusterId,
    range:
      fromParam && toParam
        ? {
            from: new Date(fromParam),
            to: new Date(toParam),
            isCustom: true,
          }
        : undefined,
    setRange,
    search,
    setSearch,
    sort,
    setSort,
    ctx,
  };
}

export type State = ReturnType<typeof useAuditEvents>;
