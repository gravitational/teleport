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

import React from 'react';

import { Box, Flex, Indicator, Text } from 'design';
import * as Icons from 'design/Icon';

import { StyledTable, StyledPanel } from './StyledTable';
import {
  BasicTableProps,
  PagedTableProps,
  SearchableBasicTableProps,
  ServersideTableProps,
  TableProps,
} from './types';
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
  emptyHint,
  emptyButton,
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

    return (
      <EmptyIndicator
        emptyText={emptyText}
        emptyHint={emptyHint}
        emptyButton={emptyButton}
        colSpan={columns.length}
      />
    );
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
        fetchStatus={fetching.fetchStatus}
      />
    );
  }

  const paginationProps: PagedTableProps<T> = {
    style,
    className,
    data: state.data as T[],
    renderHeaders,
    renderBody,
    nextPage,
    prevPage,
    pagination: state.pagination,
    searchValue: state.searchValue,
    setSearchValue,
    fetching,
  };

  if (state.pagination && state.pagination.CustomTable) {
    return <state.pagination.CustomTable {...paginationProps} />;
  }

  if (state.pagination) {
    return <PagedTable {...paginationProps} />;
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
      <InputSearch searchValue={searchValue} setSearchValue={setSearchValue} />
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
        <>
          <StyledPanel>
            <InputSearch
              searchValue={searchValue}
              setSearchValue={setSearchValue}
            />
          </StyledPanel>
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...fetching}
            {...pagination}
          />
        </>
      )}
      <StyledTable {...radiusProps} className={className} style={style}>
        {renderHeaders()}
        {renderBody(paginatedData[currentPage])}
      </StyledTable>
      {!isTopPager && (
        <StyledPanel
          borderBottomLeftRadius={3}
          borderBottomRightRadius={3}
          showTopBorder={true}
        >
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
  fetchStatus,
}: ServersideTableProps<T>) {
  return (
    <>
      {serversideProps.serversideSearchPanel}
      <StyledTable className={className} style={style}>
        {renderHeaders()}
        {renderBody(data)}
      </StyledTable>
      {(nextPage || prevPage) && (
        <StyledPanel showTopBorder={true}>
          <ServerSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            isLoading={fetchStatus === 'loading'}
          />
        </StyledPanel>
      )}
    </>
  );
}

const EmptyIndicator = ({
  emptyText,
  emptyHint,
  emptyButton,
  colSpan,
}: {
  emptyText: string;
  emptyHint: string | undefined;
  emptyButton: JSX.Element | undefined;
  colSpan: number;
}) => (
  <tfoot>
    <tr>
      <td colSpan={colSpan}>
        <Flex
          m="4"
          gap={2}
          flexDirection="column"
          alignItems="center"
          justifyContent="center"
        >
          <Flex
            gap={2}
            flexWrap="nowrap"
            alignItems="flex-start"
            justifyContent="center"
          >
            <Icons.Database
              color="text.main"
              // line-height and height must match line-height of Text below for the icon to be
              // aligned to the first line of Text if Text spans multiple lines.
              css={`
                line-height: 32px;
                height: 32px;
              `}
            />
            <Text
              textAlign="center"
              typography="paragraph"
              m="0"
              color="text.main"
            >
              {emptyText}
            </Text>
          </Flex>

          {emptyHint && (
            <Text
              textAlign="center"
              typography="paragraph"
              m="0"
              color="text.main"
            >
              {emptyHint}
            </Text>
          )}

          {emptyButton}
        </Flex>
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
