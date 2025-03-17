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

import { MatchCallback } from 'design/utils/match';

import { Pagination } from './useTable';

export type TableProps<T> = {
  data: T[];
  columns: TableColumn<T>[];
  emptyText: string;
  /**
   * Optional button that is rendered below emptyText if there's no data, during processing or on
   * error.
   */
  emptyButton?: JSX.Element;
  /**
   * Optional hint that is rendered below emptyText if there's no data, during processing or on
   * error.
   */
  emptyHint?: string;
  pagination?: PaginationConfig<T>;
  /**
   * config for client searching.
   * supports any table except when "serversideProps"
   * field is defined
   */
  clientSearch?: {
    /**
     * By default, no initial search is applied (empty search),
     * unless "initialSearchValue" is defined.
     */
    initialSearchValue: string;
    /**
     * After setting a new search value, this function will be called.
     */
    onSearchValueChange(searchString: string): void;
  };
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
  /**
   * row configuration
   */
  row?: {
    onClick?(row: T): void;
    /**
     * conditionally style a row (eg: cursor: pointer, disabled)
     */
    getStyle?(row: T): React.CSSProperties;
    /**
     * conditionally render a custom row
     * use case: by default all columns are represented by cells
     * but certain rows you need all the columns to be merged
     * into one cell to render other related elements like a
     * dropdown selector.
     */
    customRow?(row: T): JSX.Element;
  };
};

type TableColumnBase<T> = {
  headerText?: string;
  render?: (row: T) => JSX.Element;
  isSortable?: boolean;
  onSort?: (a: T, b: T) => number;
  // isNonRender is a flag that when true,
  // does not render the column or cell in table.
  // Use case: when a column combines two
  // fields but still needs both field to be searchable.
  isNonRender?: boolean;
};

export type PagerPosition = 'top' | 'bottom' | 'both';

export type PaginationConfig<T> = {
  pageSize?: number;
  /**
   * "undefined" will show both pagers if data on current page is some
   * sufficient length.
   *
   * Otherwise, it will only show the top pager.
   *
   * "both" will show both regardless of data length.
   */
  pagerPosition?: PagerPosition;
  CustomTable?: (p: PagedTableProps<T>) => JSX.Element;
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
  key: keyof T & string;
  /**
   * altSortKey is the alternative field to sort column by,
   * if provided.
   * Otherwise, it falls back to sorting by field "key".
   * @deprecated Provide the custom sorting logic through `onSort` function.
   */
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

export type BasicTableProps<T> = {
  data: T[];
  renderHeaders: () => JSX.Element;
  renderBody: (data: T[], rowOnClick?: (d: any) => void) => JSX.Element;
  className?: string;
  style?: React.CSSProperties;
};

export type SearchableBasicTableProps<T> = BasicTableProps<T> & {
  searchValue: string;
  setSearchValue: (searchValue: string) => void;
};

export type PagedTableProps<T> = SearchableBasicTableProps<T> & {
  nextPage: () => void;
  prevPage: () => void;
  pagination: Pagination<T>;
  fetching?: FetchingConfig;
  isSearchable?: boolean;
};

export type ServersideTableProps<T> = BasicTableProps<T> & {
  nextPage?: () => void;
  prevPage?: () => void;
  pagination?: Pagination<T>;
  serversideProps: ServersideProps;
  fetchStatus?: FetchStatus;
};
