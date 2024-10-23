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

import { useEffect, useState } from 'react';

import isMatch, { MatchCallback } from 'design/utils/match';

import paginateData from './Pager/paginateData';
import {
  TableProps,
  TableColumn,
  PagerPosition,
  PagedTableProps,
} from './types';

export default function useTable<T>({
  data,
  columns,
  pagination,
  showFirst,
  clientSearch,
  searchableProps,
  customSearchMatchers = [],
  serversideProps,
  fetching,
  customSort,
  disableFilter = false,
  ...props
}: TableProps<T>) {
  const [state, setState] = useState<{
    data: T[];
    searchValue: string;
    sort?: AppliedSort<T>;
    pagination?: AppliedPagination<T>;
  }>(() => {
    // Finds the first sortable column to use for the initial sorting
    let col: TableColumn<T> | undefined;
    if (!customSort) {
      const { initialSort } = props;
      if (initialSort) {
        col = initialSort.altSortKey
          ? columns.find(col => col.altSortKey === initialSort.altSortKey)
          : columns.find(col => col.key === initialSort.key);
      } else {
        col = columns.find(column => column.isSortable);
      }
    }

    return {
      data: [],
      searchValue: clientSearch?.initialSearchValue || '',
      sort: col
        ? {
            key: (col.altSortKey || col.key) as keyof T,
            onSort: col.onSort,
            dir: props.initialSort?.dir || 'ASC',
          }
        : undefined,
      pagination: pagination
        ? {
            paginatedData: paginateData([], pagination.pageSize),
            currentPage: 0,
            pagerPosition: pagination.pagerPosition,
            pageSize: pagination.pageSize || 15,
            CustomTable: pagination.CustomTable,
          }
        : undefined,
    };
  });

  function searchAndFilterCb(
    targetValue: any,
    searchValue: string,
    propName: keyof T & string
  ) {
    for (const matcher of customSearchMatchers) {
      const isMatched = matcher(targetValue, searchValue, propName);
      if (isMatched) {
        return true;
      }
    }

    // No match found.
    return false;
  }

  const updateData = (
    sort: AppliedSort<T> | undefined,
    searchValue: string,
    resetCurrentPage = false
  ) => {
    // Don't do clientside sorting and filtering if serversideProps are defined
    const sortedAndFiltered = serversideProps
      ? data
      : sortAndFilter(
          data,
          searchValue,
          sort,
          searchableProps ||
            (columns
              .filter(column => column.key)
              .map(column => column.key) as (keyof T & string)[]),
          searchAndFilterCb,
          showFirst
        );
    if (pagination && !serversideProps) {
      const pages = paginateData(sortedAndFiltered, pagination.pageSize);
      // Preserve the current page, instead of resetting it every data update.
      // The currentPage index can be out of range if data were deleted
      // from the original data. If that were the case, reset the currentPage
      // to the last page.
      let currentPage = state.pagination?.currentPage || 0;
      if (resetCurrentPage) {
        // Resetting the current page is desirable when user is newly sorting
        // or entered a new search.
        currentPage = 0;
      } else if (currentPage && pages.length > 0 && !pages[currentPage]) {
        currentPage = pages.length - 1;
      }
      setState({
        ...state,
        sort,
        searchValue,
        data: sortedAndFiltered,
        pagination: {
          ...state.pagination,
          currentPage,
          paginatedData: pages,
        },
      });
    } else {
      setState({
        ...state,
        sort,
        searchValue,
        data: sortedAndFiltered,
      });
    }
  };

  function onSort(column: TableColumn<T>) {
    if (customSort) {
      customSort.onSort({
        // @ts-expect-error TODO(gzdunek): The key can be undefined since the column can provide altKey. Improve the types.
        fieldName: column.key,
        dir: customSort.dir === 'ASC' ? 'DESC' : 'ASC',
      });
      return;
    }

    updateData(
      {
        key: (column.altSortKey || column.key) as keyof T,
        onSort: column.onSort,
        dir: state.sort?.dir === 'ASC' ? 'DESC' : 'ASC',
      },
      state.searchValue,
      true /* resetCurrentPage */
    );
  }

  function setSearchValue(searchValue: string) {
    updateData(state.sort, searchValue, true /* resetCurrentPage */);
    if (clientSearch?.onSearchValueChange) {
      clientSearch.onSearchValueChange(searchValue);
    }
  }

  function nextPage() {
    if (serversideProps) {
      fetching?.onFetchNext?.();
    }
    setState({
      ...state,
      pagination: state.pagination
        ? {
            ...state.pagination,
            currentPage: state.pagination.currentPage + 1,
          }
        : undefined,
    });
  }

  function prevPage() {
    if (serversideProps) {
      fetching?.onFetchPrev?.();
    }
    setState({
      ...state,
      pagination: state.pagination
        ? {
            ...state.pagination,
            currentPage: state.pagination.currentPage + 1,
          }
        : undefined,
    });
  }

  useEffect(() => {
    if (serversideProps || disableFilter) {
      setState({
        ...state,
        data,
      });
    } else {
      updateData(state.sort, state.searchValue);
    }
  }, [data, serversideProps]);

  return {
    state,
    columns,
    setState,
    setSearchValue,
    onSort,
    nextPage,
    prevPage,
    fetching,
    serversideProps,
    customSort,
    clientSearch,
    ...props,
  };
}

function sortAndFilter<T>(
  data: T[] = [],
  searchValue = '',
  sort: AppliedSort<T> | undefined,
  searchableProps: (keyof T & string)[],
  searchAndFilterCb: MatchCallback<T>,
  showFirst?: TableProps<T>['showFirst']
) {
  const output = data.filter(obj =>
    isMatch(obj, searchValue, {
      searchableProps,
      cb: searchAndFilterCb,
    })
  );
  if (sort) {
    const { onSort } = sort;
    if (onSort) {
      output.sort((a, b) => onSort(a, b));
    } else {
      output.sort((a, b) => {
        const aValue = a[sort.key];
        const bValue = b[sort.key];

        if (isISODate(aValue) && isISODate(bValue)) {
          return new Date(aValue).getTime() - new Date(bValue).getTime();
        }

        if (typeof aValue === 'string' && typeof bValue === 'string') {
          return aValue.localeCompare(bValue, undefined, { numeric: true });
        }

        // + operator converts to a number
        return +aValue - +bValue;
      });
    }

    if (sort.dir === 'DESC') {
      output.reverse();
    }
  }

  if (showFirst) {
    const index = output.indexOf(showFirst(data));
    if (index !== -1) {
      const item = output[index];
      output.splice(index, 1);
      output.unshift(item);
    }
  }

  return output;
}

function isISODate(value: unknown): value is Date {
  if (typeof value !== 'string') {
    return false;
  }

  const date = new Date(value);

  return !isNaN(date.getTime());
}

export type AppliedSort<T> = {
  key: keyof T;
  onSort?(a: T, b: T): number;
  dir: 'ASC' | 'DESC';
};

export type AppliedPagination<T> = {
  paginatedData: T[][];
  currentPage: number;
  pagerPosition?: PagerPosition;
  pageSize?: number;
  CustomTable?: (p: PagedTableProps<T>) => JSX.Element;
};
