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
