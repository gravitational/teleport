/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useEffect, useMemo } from 'react';
import { FetchStatus } from 'design/DataTable/types';

import { UrlResourcesParams } from 'teleport/config';

import { TableWrapper, HybridListProps } from '../common';
import { CommonListProps, LockResourceKind } from '../../common';

export type HybridListOpts = {
  getFetchFn(selectedResourceKind: LockResourceKind): (p: any) => Promise<any>;
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
