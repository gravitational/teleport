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

import React, { useState, useEffect, useMemo } from 'react';
import { FetchStatus } from 'design/DataTable/types';

import { UrlResourcesParams } from 'teleport/config';

import { TableWrapper, HybridListProps } from '../common';
import { CommonListProps, LockResourceKind } from '../../common';

export type HybridListOpts = {
  getFetchFn(
    selectedResourceKind: LockResourceKind
  ): (p: any, signal?: AbortSignal) => Promise<any>;
  getTable(
    selectedResourceKind: LockResourceKind,
    resources: any[],
    listProps: HybridListProps
  ): React.ReactElement;
};

export function HybridList(props: CommonListProps & { opts: HybridListOpts }) {
  const [tableData, setTableData] = useState<TableData>(emptyTableData);

  function fetchNextPage() {
    fetch({ ...tableData });
  }

  function fetch(data: TableData) {
    const fetchFn = data.fetchFn;
    setTableData({ ...data, fetchStatus: 'loading' });
    props.setAttempt({ status: 'processing' });

    fetchFn({ startKey: data.startKey, limit: props.pageSize })
      .then(resp => {
        props.setAttempt({ status: 'success' });
        setTableData({
          fetchFn,
          startKey: resp.startKey,
          fetchStatus: resp.startKey ? '' : 'disabled',
          // concat each page fetch.
          resources: [...data.resources, ...resp.items],
        });
      })
      .catch((err: Error) => {
        props.setAttempt({ status: 'failed', statusText: err.message });
        setTableData({ ...data, fetchStatus: '' }); // fallback to previous data
      });
  }

  // Load the correct function to use for init fetch and
  // for next pages.
  useEffect(() => {
    let fetchFn;
    switch (props.selectedResourceKind) {
      default:
        fetchFn = props.opts?.getFetchFn(props.selectedResourceKind);
        if (!fetchFn) {
          console.error(
            `[HybridList.tsx] fetchFn not defined for resource kind ${props.selectedResourceKind}`
          );
          return; // don't do anything on error
        }
    }

    // Reset table per selected resource change.
    fetch({ ...emptyTableData, fetchFn });
  }, [props.selectedResourceKind]);

  const table = useMemo(() => {
    const listProps: HybridListProps = {
      pageSize: props.pageSize,
      selectedResources: props.selectedResources,
      toggleSelectResource: props.toggleSelectResource,
      fetchNextPage,
      fetchStatus: tableData.fetchStatus,
    };
    switch (props.selectedResourceKind) {
      default:
        const table = props.opts?.getTable(
          props.selectedResourceKind,
          tableData.resources,
          listProps
        );
        if (table) {
          return table;
        }
        console.error(
          `[HybridList.tsx] table not defined for resource kind ${props.selectedResourceKind}`
        );
    }
  }, [props.attempt, tableData, props.selectedResources]);

  return (
    <TableWrapper
      className={props.attempt.status === 'processing' ? 'disabled' : ''}
    >
      {table}
    </TableWrapper>
  );
}

const emptyTableData: TableData = {
  resources: [],
  fetchStatus: 'loading',
  startKey: '',
};

type TableData = {
  resources: any[];
  fetchStatus: FetchStatus;
  startKey: string;
  fetchFn?(params?: UrlResourcesParams): Promise<any>;
};
