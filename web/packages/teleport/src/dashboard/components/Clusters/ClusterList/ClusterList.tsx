/*
Copyright 2019-2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';
import isMatch from 'design/utils/match';
import history from 'teleport/services/history';
import { Cluster } from 'teleport/services/clusters';
import { sortBy } from 'lodash';
import {
  SortHeaderCell,
  TextCell,
  Cell,
  Table,
  Column,
  SortTypes,
} from 'design/DataTable';
import { usePages, Pager, StyledPanel } from 'design/DataTable/Paged';
import { ButtonBorder } from 'design';
import * as Labels from 'design/Label';
import cfg from 'teleport/config';

export default function ClustersList(props: ClusterListProps) {
  const { clusters, search = '', pageSize = 50 } = props;
  const [sorting, setSorting] = React.useState<Sorting>({});

  function onSortChange(columnKey: SortCol, sortDir: string) {
    setSorting({ [columnKey]: sortDir });
  }

  function sort(clusters: Cluster[]) {
    const columnName = Object.getOwnPropertyNames(sorting)[0] as SortCol;
    const sorted = sortClusters(clusters, columnName, sorting[columnName]);
    return rootFirst(sorted);
  }

  const filtered = filter(clusters, search);
  const sorted = sort(filtered);
  const paged = usePages({ pageSize, data: sorted });

  function onTableRowClick(e: MouseEvent) {
    if (e.ctrlKey || e.shiftKey || e.altKey) {
      return;
    }

    const closest = (e.target as any).closest('a, button, tbody tr');

    // ignore clicks on input elements (a, buttons)
    if (!closest || closest.tagName !== 'TR') {
      return;
    }

    const rows = closest.parentElement.childNodes;
    for (var i = 0; i < rows.length; i++) {
      if (rows[i] === closest) {
        history.push(paged.data[i].url);
      }
    }
  }

  // add empty rows for decorative purposes
  if (filtered.length === clusters.length) {
    for (let i = paged.data.length; i < 5; i++) {
      paged.data.push({});
    }
  }

  return (
    <>
      <StyledPanel
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
        justifyContent="space-between"
      >
        <Pager {...paged} />
      </StyledPanel>
      <ClusterTable data={paged.data} onClick={onTableRowClick}>
        <Column
          header={<Cell style={{ width: '40px' }} />}
          cell={<RootLabelCell />}
        />
        <Column
          columnKey="clusterId"
          header={
            <SortHeaderCell
              sortDir={sorting.clusterId}
              onSortChange={onSortChange}
              title="Name"
            />
          }
          cell={<NameCell />}
        />
        <Column
          columnKey="authVersion"
          header={
            <SortHeaderCell
              sortDir={sorting.authVersion}
              onSortChange={onSortChange}
              title="Version"
            />
          }
          cell={<TextCell />}
        />
        <Column
          columnKey="nodeCount"
          header={
            <SortHeaderCell
              sortDir={sorting.nodeCount}
              onSortChange={onSortChange}
              title="Nodes"
            />
          }
          cell={<TextCell />}
        />
        <Column
          columnKey="publicURL"
          header={
            <SortHeaderCell
              sortDir={sorting.publicURL}
              onSortChange={onSortChange}
              title="Public URL"
            />
          }
          cell={<TextCell />}
        />
        <Column header={<Cell />} cell={<ActionCell />} />
      </ClusterTable>
    </>
  );
}

function filter(clusters: Cluster[], searchValue = '') {
  return clusters.filter(obj =>
    isMatch(obj, searchValue, {
      searchableProps: ['clusterId', 'authVersion'],
      cb: filterCb,
    })
  );
}

function filterCb(targetValue: any[], searchValue: string, propName: string) {
  if (propName === 'labels') {
    return targetValue.some(item => {
      const { name, value } = item;
      return (
        name.toLocaleUpperCase().indexOf(searchValue) !== -1 ||
        value.toLocaleUpperCase().indexOf(searchValue) !== -1
      );
    });
  }
}

function sortClusters(clusters: Cluster[], columnName: SortCol, dir: string) {
  const sorted = sortBy(clusters, columnName);
  if (dir === SortTypes.DESC) {
    return sorted.reverse();
  }

  return sorted;
}

function rootFirst(clusters: Cluster[]) {
  const rootIndex = clusters.findIndex(c => c.clusterId === cfg.proxyCluster);
  if (rootIndex !== -1) {
    const root = clusters[rootIndex];
    clusters.splice(rootIndex, 1);
    clusters.unshift(root);
  }
  return clusters;
}

export function NameCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];
  return (
    <Cell>
      <strong>{clusterId}</strong>
    </Cell>
  );
}

function RootLabelCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];
  const isRoot = cfg.proxyCluster === clusterId;
  return <Cell>{isRoot && <Labels.Primary>Root</Labels.Primary>}</Cell>;
}

function ActionCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];

  if (!clusterId) {
    return <Cell />;
  }

  const url = cfg.getClusterRoute(clusterId);
  return (
    <Cell align="right">
      <ButtonBorder as={NavLink} to={url} size="small">
        View
      </ButtonBorder>
    </Cell>
  );
}

type SortCol = keyof Cluster;
type Sorting = {
  [P in keyof Cluster]?: string;
};

type ClusterListProps = {
  clusters: Cluster[];
  search: string;
  pageSize?: 500;
};

const ClusterTable = styled(Table)`
  td {
    padding 4px 24px !important;
    height: 22px;
  }

  tbody tr {
    border-bottom: 1px solid ${props => props.theme.colors.primary.main};
  }

  tbody tr:hover {
    cursor: pointer;
    background-color: ${props => props.theme.colors.primary.lighter};
    border-bottom: 1px solid ${props => props.theme.colors.primary.lighter};
  }
`;
