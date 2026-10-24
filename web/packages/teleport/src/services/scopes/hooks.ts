/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import {
  InfiniteData,
  keepPreviousData,
  useInfiniteQuery,
  UseInfiniteQueryResult,
} from '@tanstack/react-query';

import { listAssignedScopes, ListAssignedScopesResponse } from './scopes';

export function useAssignedScopes({
  prefix = '',
  enabled = true,
  retryOnMount,
}: {
  prefix?: string;
  enabled?: boolean;
  retryOnMount?: boolean;
} = {}): UseInfiniteQueryResult<
  InfiniteData<ListAssignedScopesResponse>,
  Error
> {
  if (!prefix.startsWith('/')) {
    prefix = '/' + prefix;
  }
  return useInfiniteQuery({
    queryKey: ['scopes', prefix],
    queryFn: ({ signal, pageParam }) =>
      listAssignedScopes({ pageToken: pageParam, prefix }, signal),
    initialPageParam: '',
    getNextPageParam: lastPage => lastPage.nextPageToken,
    placeholderData: keepPreviousData,
    retryOnMount,
    enabled,
  });
}
