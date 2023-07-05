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

import React from 'react';

import { Text, Indicator, Box } from 'design';
import * as Icons from 'design/Icon';

import { StyledTable, StyledPanel } from './StyledTable';
import { TableProps } from './types';
import { SortHeaderCell, TextCell } from './Cells';
import { ClientSidePager, ServerSidePager } from './Pager';
import InputSearch from './InputSearch';
import useTable, { State } from './useTable';

export default function Container<T>(props: TableProps<T>) {
  const tableProps = useTable(props);
  return <Table<T> {...tableProps} />;
}

export function Table<T>({
  columns,
  state,
  onSort,
  emptyText,
  nextPage,
  prevPage,
  setSearchValue,
  isSearchable,
  fetching,
  className,
  style,
  serversideProps,
  customSort,
}: State<T>) {
  const renderHeaders = () => {
    const headers = columns.flatMap(column => {
      if (column.isNonRender) {
        return []; // does not include this column.
      }

      const headerText = column.headerText || '';

      let dir;
      if (customSort) {
        dir = customSort.fieldName == column.key ? customSort.dir : null;
      } else {
        dir =
          state.sort?.key === column.key ||
          state.sort?.key === column.altSortKey
            ? state.sort?.dir
            : null;
      }

      const $cell = column.isSortable ? (
        <SortHeaderCell<T>
          column={column}
          serversideProps={serversideProps}
          text={headerText}
          onClick={() => onSort(column)}
          dir={dir}
        />
      ) : (
        <th style={{ cursor: 'default' }}>{headerText}</th>
      );

      return (
        <React.Fragment key={column.key || column.altKey}>
          {$cell}
        </React.Fragment>
      );
    });

    return (
      <thead>
        <tr>{headers}</tr>
      </thead>
    );
  };

  const renderBody = (data: T[]) => {
    const rows = [];

    if (fetching?.fetchStatus === 'loading') {
      return <LoadingIndicator colSpan={columns.length} />;
    }
    data.map((item, rowIdx) => {
      const cells = columns.flatMap((column, columnIdx) => {
        if (column.isNonRender) {
          return []; // does not include this column.
        }

        const $cell = column.render ? (
          column.render(item)
        ) : (
          <TextCell data={item[column.key]} />
        );

        return (
          <React.Fragment key={`${rowIdx} ${columnIdx}`}>
            {$cell}
          </React.Fragment>
        );
      });
      rows.push(<tr key={rowIdx}>{cells}</tr>);
    });

    if (rows.length) {
      return <tbody>{rows}</tbody>;
    }

    return <EmptyIndicator emptyText={emptyText} colSpan={columns.length} />;
  };

  if (serversideProps) {
    return (
      <ServersideTable
        style={style}
        className={className}
        data={state.data}
        renderHeaders={renderHeaders}
        renderBody={renderBody}
        nextPage={fetching.onFetchNext}
        prevPage={fetching.onFetchPrev}
        pagination={state.pagination}
        serversideProps={serversideProps}
      />
    );
  }

  if (state.pagination) {
    return (
      <PagedTable
        style={style}
        className={className}
        data={state.data}
        renderHeaders={renderHeaders}
        renderBody={renderBody}
        nextPage={nextPage}
        prevPage={prevPage}
        pagination={state.pagination}
        searchValue={state.searchValue}
        setSearchValue={setSearchValue}
        fetching={fetching}
      />
    );
  }

  if (isSearchable) {
    return (
      <SearchableBasicTable
        style={style}
        className={className}
        data={state.data}
        renderHeaders={renderHeaders}
        renderBody={renderBody}
        searchValue={state.searchValue}
        setSearchValue={setSearchValue}
      />
    );
  }

  return (
    <BasicTable
      style={style}
      className={className}
      data={state.data}
      renderHeaders={renderHeaders}
      renderBody={renderBody}
    />
  );
}

function BasicTable<T>({
  data,
  renderHeaders,
  renderBody,
  className,
  style,
}: BasicTableProps<T>) {
  return (
    <StyledTable className={className} style={style}>
      {renderHeaders()}
      {renderBody(data)}
    </StyledTable>
  );
}

function SearchableBasicTable<T>({
  data,
  renderHeaders,
  renderBody,
  searchValue,
  setSearchValue,
  className,
  style,
}: SearchableBasicTableProps<T>) {
  return (
    <>
      <StyledPanel borderTopLeftRadius={3} borderTopRightRadius={3}>
        <InputSearch
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
      </StyledPanel>
      <StyledTable
        className={className}
        borderTopLeftRadius={0}
        borderTopRightRadius={0}
        style={style}
      >
        {renderHeaders()}
        {renderBody(data)}
      </StyledTable>
    </>
  );
}

function PagedTable<T>({
  nextPage,
  prevPage,
  renderHeaders,
  renderBody,
  data,
  pagination,
  searchValue,
  setSearchValue,
  fetching,
  className,
  style,
}: PagedTableProps<T>) {
  const { pagerPosition, paginatedData, currentPage } = pagination;
  const isTopPager = pagerPosition === 'top';

  const radiusProps = {
    borderTopLeftRadius: 3,
    borderTopRightRadius: 3,
    borderBottomLeftRadius: 3,
    borderBottomRightRadius: 3,
  };

  if (isTopPager) {
    radiusProps.borderTopLeftRadius = 0;
    radiusProps.borderTopRightRadius = 0;
  } else {
    radiusProps.borderBottomLeftRadius = 0;
    radiusProps.borderBottomRightRadius = 0;
  }
  return (
    <>
      {isTopPager && (
        <StyledPanel borderTopLeftRadius={3} borderTopRightRadius={3}>
          <InputSearch
            searchValue={searchValue}
            setSearchValue={setSearchValue}
          />
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...fetching}
            {...pagination}
          />
        </StyledPanel>
      )}
      <StyledTable {...radiusProps} className={className} style={style}>
        {renderHeaders()}
        {renderBody(paginatedData[currentPage])}
      </StyledTable>
      {!isTopPager && (
        <StyledPanel borderBottomLeftRadius={3} borderBottomRightRadius={3}>
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...pagination}
          />
        </StyledPanel>
      )}
    </>
  );
}

function ServersideTable<T>({
  nextPage,
  prevPage,
  renderHeaders,
  renderBody,
  data,
  className,
  style,
  serversideProps,
}: ServersideTableProps<T>) {
  return (
    <>
      {serversideProps.serversideSearchPanel}
      <StyledTable className={className} style={style}>
        {renderHeaders()}
        {renderBody(data)}
      </StyledTable>
      <StyledPanel borderBottomLeftRadius={3} borderBottomRightRadius={3}>
        <ServerSidePager nextPage={nextPage} prevPage={prevPage} />
      </StyledPanel>
    </>
  );
}

const EmptyIndicator = ({
  emptyText,
  colSpan,
}: {
  emptyText: string;
  colSpan: number;
}) => (
  <tfoot>
    <tr>
      <td colSpan={colSpan}>
        <Text
          typography="paragraph"
          m="4"
          color="text.main"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icons.Database mr="2" />
          {emptyText}
        </Text>
      </td>
    </tr>
  </tfoot>
);

const LoadingIndicator = ({ colSpan }: { colSpan: number }) => (
  <tfoot>
    <tr>
      <td colSpan={colSpan}>
        <Box m={4} textAlign="center">
          <Indicator delay="none" />
        </Box>
      </td>
    </tr>
  </tfoot>
);

type BasicTableProps<T> = {
  data: T[];
  renderHeaders: () => JSX.Element;
  renderBody: (data: T[]) => JSX.Element;
  className?: string;
  style?: React.CSSProperties;
};

type SearchableBasicTableProps<T> = BasicTableProps<T> & {
  searchValue: string;
  setSearchValue: (searchValue: string) => void;
};

type PagedTableProps<T> = SearchableBasicTableProps<T> & {
  nextPage: () => void;
  prevPage: () => void;
  pagination: State<T>['state']['pagination'];
  fetching?: State<T>['fetching'];
};

type ServersideTableProps<T> = BasicTableProps<T> & {
  nextPage: () => void;
  prevPage: () => void;
  pagination: State<T>['state']['pagination'];
  serversideProps: State<T>['serversideProps'];
};
