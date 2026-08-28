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

import type { FetchStatus } from '../../types';

export function useClientSidePager({
  data = [],
  paginatedData = [],
  currentPage = 0,
  pageSize = 50,
}: Props): State {
  const currentPageData = paginatedData[currentPage] || [];
  const searchFrom = currentPage * pageSize;

  const from = data.indexOf(currentPageData[0], searchFrom);
  const to = data.lastIndexOf(
    currentPageData[currentPageData.length - 1],
    searchFrom + pageSize - 1
  );

  const count = data.length;

  const isNextDisabled = to === data.length - 1;
  const isPrevDisabled = currentPage === 0;

  return {
    from,
    to,
    count,
    isNextDisabled,
    isPrevDisabled,
  };
}

export type Props = {
  nextPage: () => void;
  prevPage: () => void;
  data: any[];
  paginatedData?: Array<Array<any>>;
  currentPage?: number;
  pageSize?: number;
  onFetchMore?: () => void;
  fetchStatus?: FetchStatus;
};

export type State = {
  from: number;
  to: number;
  count: number;
  isNextDisabled: boolean;
  isPrevDisabled: boolean;
};
