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

import { MatchCallback } from 'design/utils/match';

export type TableProps<T> = {
  data: T[];
  columns: TableColumn<T>[];
  emptyText: string;
  pagination?: PaginationConfig;
  isSearchable?: boolean;
  searchableProps?: Extract<keyof T, string>[];
  // customSearchMatchers contains custom functions to run when search matching.
  // 'targetValue' prop will have to be uppercased for proper matching since
  // the root matcher will uppercase the searchValue.
  customSearchMatchers?: MatchCallback<T>[];
  initialSort?: InitialSort<T>;
  serversideProps?: ServersideProps;
  fetching?: FetchingConfig;
  showFirst?: (data: T[]) => T;
  className?: string;
  style?: React.CSSProperties;
  // customSort contains fields that describe the current sort direction,
  // the field to sort by, and a custom sort function.
  customSort?: CustomSort;
  // disableFilter when true means to skip running
  // any client table filtering supplied by default.
  // Use case: filtering is done on the caller side e.g. server side.
  disableFilter?: boolean;
};

type TableColumnBase<T> = {
  headerText?: string;
  render?: (row: T) => JSX.Element;
  isSortable?: boolean;
  onSort?: (a, b) => number;
  // isNonRender is a flag that when true,
  // does not render the column or cell in table.
  // Use case: when a column combines two
  // fields but still needs both field to be searchable.
  isNonRender?: boolean;
};

export type PaginationConfig = {
  pageSize?: number;
  pagerPosition?: 'top' | 'bottom';
};

/**
 * Page keeps track of our current agent list
 * start keys and the current position.
 */
export type Page = {
  /** Keys are the list of start keys collected from each page fetch. */
  keys: string[];
  /**
   * Index refers to the current index the page
   * is at in the list of keys. Eg. an index of 1
   * would mean that we are on the second key
   * and thus on the second page.
   */
  index: number;
};

export type FetchingConfig = {
  onFetchNext?: () => void;
  onFetchPrev?: () => void;
  onFetchMore?: () => void;
  fetchStatus: FetchStatus;
};

export type ServersideProps = {
  serversideSearchPanel: JSX.Element;
  sort: SortType;
  setSort: (sort: SortType) => void;
};

// Makes it so either key or altKey is required
type TableColumnWithKey<T> = TableColumnBase<T> & {
  key: Extract<keyof T, string>;
  // altSortKey is the alternative field to sort column by,
  // if provided. Otherwise it falls back to sorting by field
  // "key".
  altSortKey?: Extract<keyof T, string>;
  altKey?: never;
};

type TableColumnWithAltKey<T> = TableColumnBase<T> & {
  altKey: string;
  key?: never;
  altSortKey?: never;
};

// InitialSort defines the field (table column) that should be initiallly
// sorted on render. If not provided, it defaults to finding the first
// sortable column.

// Either "key" or "altSortKey" can be provided
// but not both. If "altSortKey" is provided, than that TableColumn
// should also define "altSortKey" (TableColumnWithAltKey).
type InitialSort<T> = {
  dir: SortDir;
} & (
  | { key: Extract<keyof T, string>; altSortKey?: never }
  | { altSortKey: Extract<keyof T, string>; key?: never }
);

export type SortType = {
  fieldName: string;
  dir: SortDir;
};

export type SortDir = 'ASC' | 'DESC';

export type FetchStatus = 'loading' | 'disabled' | '';

export type TableColumn<T> = TableColumnWithKey<T> | TableColumnWithAltKey<T>;

export type LabelDescription = {
  name: string;
  value: string;
};

export type CustomSort = SortType & {
  onSort(s: SortType): void;
};
