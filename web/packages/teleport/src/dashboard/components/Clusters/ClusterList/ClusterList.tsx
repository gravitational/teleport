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
import styled from 'styled-components';
import isMatch from 'design/utils/match';
import history from 'teleport/services/history';
import { Cluster } from 'teleport/services/clusters';
import { sortBy } from 'lodash';
import {
  TablePaged,
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
} from 'design/DataTable';
import { ButtonSecondary, Text } from 'design';
import { NavLink } from 'shared/components/Router';
import * as Labels from 'design/Label';
import cfg from 'teleport/config';

export default function ClustersList(props: ClusterListProps) {
  const { clusters, search = '', pageSize = 500 } = props;
  const [sortDir, setSortDir] = React.useState<ClusterProps>({});

  function onSortChange(columnKey, sortDir) {
    setSortDir({ [columnKey]: sortDir });
  }

  function sort(clusters: Cluster[]) {
    const columnKey = Object.getOwnPropertyNames(sortDir)[0];
    const sorted = sortBy(clusters, columnKey);
    if (sortDir[columnKey] === SortTypes.DESC) {
      return sortByRoot(sorted.reverse());
    }

    return clusters;
  }

  const filtered = filter(clusters, search);
  const data = sort(filtered);
  const tableProps = {
    pageSize,
    data,
  };

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
        history.push(clusters[i].url);
      }
    }
  }

  return (
    <StyledTable {...tableProps} onClick={onTableRowClick}>
      <Column
        header={<Cell style={{ width: '40px' }} />}
        cell={<RootLabelCell />}
      />
      <Column
        columnKey="clusterId"
        header={
          <SortHeaderCell
            sortDir={sortDir.clusterId}
            onSortChange={onSortChange}
            title="Name"
          />
        }
        cell={<NameCell />}
      />
      <Column
        columnKey="version"
        header={
          <SortHeaderCell
            sortDir={sortDir.version}
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
            sortDir={sortDir.nodeCount}
            onSortChange={onSortChange}
            title="Nodes"
          />
        }
        cell={<TextCell />}
      />
      <Column
        columnKey="publicUrl"
        header={
          <SortHeaderCell
            sortDir={sortDir.publicUrl}
            onSortChange={onSortChange}
            title="Public URL"
          />
        }
        cell={<TextCell />}
      />
      <Column header={<Cell />} cell={<ActionCell />} />
    </StyledTable>
  );
}

function filter(clusters: Cluster[], searchValue = '') {
  return clusters.filter(obj =>
    isMatch(obj, searchValue, {
      searchableProps: ['clusterId', 'version'],
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

function sortByRoot(clusters: Cluster[]) {
  const proxyCluster = clusters.find(c => c.clusterId === cfg.proxyCluster);
  if (proxyCluster) {
    const tmp = clusters[0];
    const index = clusters.indexOf(proxyCluster);
    clusters[0] = clusters[index];
    clusters[index] = tmp;
  }
  return clusters;
}

export function NameCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];
  return (
    <Cell>
      <Text typography="h6">{clusterId}</Text>
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
  const { url } = data[rowIndex];
  return (
    <Cell align="right">
      <ButtonSecondary
        as={NavLink}
        to={url}
        size="small"
        width="90px"
        children="view"
      />
    </Cell>
  );
}

const StyledTable = styled(TablePaged)`
  tr:hover {
    cursor: pointer;
    background-color: rgba(0, 0, 0, 0.075);
  }
`;

type ClusterProps = {
  [P in keyof Cluster]?: string;
};

type ClusterListProps = {
  clusters: Cluster[];
  search: string;
  pageSize?: 500;
};
