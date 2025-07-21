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

import React, { PropsWithChildren, ReactNode, type JSX } from 'react';

import { Box, Flex, Indicator, P1, Text } from 'design';
import * as Icons from 'design/Icon';

import { SortHeaderCell, TextCell } from './Cells';
import InputSearch from './InputSearch';
import { ClientSidePager, ServerSidePager } from './Pager';
import { StyledPanel, StyledTable } from './StyledTable';
import {
  BasicTableProps,
  PagedTableProps,
  PagerPosition,
  SearchableBasicTableProps,
  ServersideTableProps,
  SortDir,
  TableProps,
} from './types';
import useTable from './useTable';

export default function Table<T>(props: TableProps<T>) {
  const {
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
    row,
  } = useTable(props);

  const renderHeaders = () => {
    const headers = columns.flatMap(column => {
      if (column.isNonRender) {
        return []; // does not include this column.
      }

      const headerText = column.headerText || '';

      let dir: SortDir | undefined;
      if (customSort) {
        dir = customSort.fieldName == column.key ? customSort.dir : undefined;
      } else {
        dir =
          state.sort?.key === column.key ||
          state.sort?.key === column.altSortKey
            ? state.sort?.dir
            : undefined;
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
    const rows: ReactNode[] = [];

    if (
      fetching?.fetchStatus === 'loading' &&
      !fetching.disableLoadingIndicator
    ) {
      return <LoadingIndicator colSpan={columns.length} />;
    }
    data.map((item, rowIdx) => {
      const TableRow: React.FC<PropsWithChildren> = ({ children }) => (
        <tr
          key={rowIdx}
          onClick={() => row?.onClick?.(item)}
          style={row?.getStyle?.(item)}
        >
          {children}
        </tr>
      );

      const customRow = row?.customRow?.(item);
      if (customRow) {
        rows.push(<TableRow key={rowIdx}>{customRow}</TableRow>);
        return;
      }

      const cells = columns.flatMap((column, columnIdx) => {
        if (column.isNonRender) {
          return []; // does not include this column.
        }

        const $cell = column.render ? (
          column.render(item)
        ) : (
          <TextCell data={column.key ? item[column.key] : undefined} />
        );

        return (
          <React.Fragment key={`${rowIdx} ${columnIdx}`}>
            {$cell}
          </React.Fragment>
        );
      });
      rows.push(<TableRow key={rowIdx}>{cells}</TableRow>);
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
        nextPage={fetching?.onFetchNext}
        prevPage={fetching?.onFetchPrev}
        pagination={state.pagination}
        serversideProps={serversideProps}
        fetchStatus={fetching?.fetchStatus}
      />
    );
  }

  if (state.pagination) {
    const paginationProps: PagedTableProps<T> = {
      style,
      className,
      data: state.data,
      renderHeaders,
      renderBody,
      nextPage,
      prevPage,
      pagination: state.pagination,
      searchValue: state.searchValue,
      setSearchValue,
      fetching,
      isSearchable,
    };

    if (state.pagination.CustomTable) {
      return <state.pagination.CustomTable {...paginationProps} />;
    }

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
      <Box mb={3}>
        <InputSearch
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
      </Box>
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
  isSearchable,
}: PagedTableProps<T>) {
  const { pagerPosition, paginatedData, currentPage } = pagination;
  const { showBothPager, showBottomPager, showTopPager } = getPagerPosition(
    pagerPosition,
    paginatedData[currentPage].length
  );

  return (
    <>
      {(isSearchable || showTopPager || showBothPager) && (
        <StyledPanel>
          {isSearchable && (
            <InputSearch
              searchValue={searchValue}
              setSearchValue={setSearchValue}
            />
          )}
          {(showTopPager || showBothPager) && (
            <ClientSidePager
              nextPage={nextPage}
              prevPage={prevPage}
              data={data}
              {...fetching}
              {...pagination}
            />
          )}
        </StyledPanel>
      )}
      <StyledTable className={className} style={style}>
        {renderHeaders()}
        {renderBody(paginatedData[currentPage])}
      </StyledTable>
      {(showBottomPager || showBothPager) && (
        <StyledPanel>
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
  pagination,
}: ServersideTableProps<T>) {
  const { showTopPager, showBothPager, showBottomPager } = getPagerPosition(
    pagination?.pagerPosition,
    data.length
  );
  return (
    <>
      <StyledPanel>
        {serversideProps?.serversideSearchPanel}
        {(showTopPager || showBothPager) && (
          <ServerSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            isLoading={fetchStatus === 'loading'}
          />
        )}
      </StyledPanel>
      <StyledTable className={className} style={style}>
        {renderHeaders()}
        {renderBody(data)}
      </StyledTable>
      {(showBottomPager || showBothPager) && (
        <Box mt={2}>
          <ServerSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            isLoading={fetchStatus === 'loading'}
          />
        </Box>
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
            gap={3}
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
            <Text textAlign="center" typography="h1" m="0" color="text.main">
              {emptyText}
            </Text>
          </Flex>

          {emptyHint && <P1 textAlign="center">{emptyHint}</P1>}

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

/**
 * Returns pager position flags.
 *
 * If pagerPosition is not defined, it defaults to:
 *   - top pager only: if current dataLen < 5
 *   - both top and bottom pager if dataLen > 5
 */
export function getPagerPosition(
  pagerPosition: PagerPosition | undefined,
  dataLen: number
) {
  const hasSufficientData = dataLen > 5;

  const showBottomPager = pagerPosition === 'bottom';
  const showTopPager =
    pagerPosition === 'top' || (!pagerPosition && !hasSufficientData);
  const showBothPager =
    pagerPosition === 'both' || (!pagerPosition && hasSufficientData);

  return { showBothPager, showBottomPager, showTopPager };
}
