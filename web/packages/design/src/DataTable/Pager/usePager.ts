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

import { FetchStatus, ServersideProps } from './../types';

export default function usePager({
  nextPage,
  prevPage,
  data = [],
  paginatedData = [],
  currentPage,
  pageSize,
  serversideProps,
  ...props
}: Props) {
  const currentPageData = paginatedData[currentPage] || [];
  const searchFrom = currentPage * pageSize;

  const from = data.indexOf(currentPageData[0], searchFrom);
  const to = data.lastIndexOf(
    currentPageData[currentPageData.length - 1],
    searchFrom + pageSize - 1
  );

  const count = data.length;

  const isNextDisabled = serversideProps
    ? serversideProps.startKeys[serversideProps.startKeys.length - 1] === ''
    : to === data.length - 1;

  const isPrevDisabled = serversideProps
    ? serversideProps.startKeys.length <= 2
    : currentPage === 0;

  return {
    nextPage,
    prevPage,
    from,
    to,
    count,
    isNextDisabled,
    isPrevDisabled,
    serversideProps,
    ...props,
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
  serversideProps?: ServersideProps;
};

export type State = ReturnType<typeof usePager>;
