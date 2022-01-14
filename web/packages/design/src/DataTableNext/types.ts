export type TableProps<T> = {
  data: T[];
  columns: TableColumn<T>[];
  emptyText: string;
  pagination?: PaginationConfig;
  isSearchable?: boolean;
  initialSort?: InitialSort<T>;
  fetching?: FetchingConfig;
  showFirst?: (data: T[]) => T;
  className?: string;
  style?: React.CSSProperties;
};

type TableColumnBase<T> = {
  headerText?: string;
  render?: (row: T) => JSX.Element;
  isSortable?: boolean;
  onSort?: (a, b) => number;
};

export type PaginationConfig = {
  pageSize?: number;
  pagerPosition?: 'top' | 'bottom';
};

export type FetchingConfig = {
  onFetchMore: () => void;
  fetchStatus: FetchStatus;
};

// Makes it so either key or altKey is required
type TableColumnWithKey<T> = TableColumnBase<T> & {
  key: Extract<keyof T, string>;
  altKey?: never;
};

type TableColumnWithAltKey<T> = TableColumnBase<T> & {
  altKey: string;
  key?: never;
};

type InitialSort<T> = {
  key: Extract<keyof T, string>;
  dir: SortDir;
};

export type SortDir = 'ASC' | 'DESC';

export type FetchStatus = 'loading' | 'disabled' | '';

export type TableColumn<T> = TableColumnWithKey<T> | TableColumnWithAltKey<T>;
