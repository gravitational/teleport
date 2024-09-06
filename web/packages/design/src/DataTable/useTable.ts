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
import { TableProps, TableColumn } from './types';

export default function useTable<T>({
  data,
  columns,
  pagination,
  showFirst,
  searchableProps,
  customSearchMatchers = [],
  serversideProps,
  fetching,
  customSort,
  disableFilter = false,
  ...props
}: TableProps<T>) {
  const [state, setState] = useState(() => {
    // Finds the first sortable column to use for the initial sorting
    let col: TableColumn<T>;
    if (!customSort) {
      if (props.initialSort) {
        col = props.initialSort.altSortKey
          ? columns.find(col => col.altSortKey === props.initialSort.altSortKey)
          : columns.find(col => col.key === props.initialSort.key);
      } else {
        col = columns.find(column => column.isSortable);
      }
    }

    return {
      data: serversideProps || disableFilter ? data : [],
      searchValue: '',
      sort: col
        ? {
            key: (col.altSortKey || col.key) as string,
            onSort: col.onSort,
            dir: props.initialSort?.dir || 'ASC',
          }
        : null,
      pagination: pagination
        ? {
            paginatedData: paginateData(data, pagination.pageSize),
            currentPage: 0,
            pagerPosition: pagination.pagerPosition,
            pageSize: pagination.pageSize || 15,
            CustomTable: pagination.CustomTable,
          }
        : null,
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
    sort: typeof state.sort,
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
            columns.filter(column => column.key).map(column => column.key),
          searchAndFilterCb,
          showFirst
        );
    if (pagination && !serversideProps) {
      const pages = paginateData(sortedAndFiltered, pagination.pageSize);
      // Preserve the current page, instead of resetting it every data update.
      // The currentPage index can be out of range if data were deleted
      // from the original data. If that were the case, reset the currentPage
      // to the last page.
      let currentPage = state.pagination.currentPage;
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

  function onSort(column: TableColumn<any>) {
    if (customSort) {
      customSort.onSort({
        fieldName: column.key,
        dir: customSort.dir === 'ASC' ? 'DESC' : 'ASC',
      });
      return;
    }

    updateData(
      {
        key: column.altSortKey || column.key,
        onSort: column.onSort,
        dir: state.sort?.dir === 'ASC' ? 'DESC' : 'ASC',
      },
      state.searchValue,
      true /* resetCurrentPage */
    );
  }

  function setSearchValue(searchValue: string) {
    updateData(state.sort, searchValue, true /* resetCurrentPage */);
  }

  function nextPage() {
    if (serversideProps) {
      fetching.onFetchNext();
    }
    setState({
      ...state,
      pagination: {
        ...state.pagination,
        currentPage: state.pagination.currentPage + 1,
      },
    });
  }

  function prevPage() {
    if (serversideProps) {
      fetching.onFetchPrev();
    }
    setState({
      ...state,
      pagination: {
        ...state.pagination,
        currentPage: state.pagination.currentPage - 1,
      },
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
    ...props,
  };
}

function sortAndFilter<T>(
  data: T[] = [],
  searchValue = '',
  sort: State<T>['state']['sort'],
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
    if (sort.onSort) {
      output.sort((a, b) => sort.onSort(a[sort.key], b[sort.key]));
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

        return aValue - bValue;
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

function isISODate(value: any): boolean {
  if (typeof value !== 'string') {
    return false;
  }

  const date = new Date(value);

  return !isNaN(date.getTime());
}

export type State<T> = Omit<
  ReturnType<typeof useTable>,
  'columns' | 'initialSort'
> & {
  columns: TableColumn<T>[];
  className?: string;
};
