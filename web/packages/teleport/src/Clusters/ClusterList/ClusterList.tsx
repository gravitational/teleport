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
import { sortBy } from 'lodash';
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';
import isMatch from 'design/utils/match';
import { Flex } from 'design';
import { Cluster } from 'teleport/services/clusters';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import {
  SortHeaderCell,
  Cell,
  Table,
  Column,
  SortTypes,
} from 'design/DataTable';
import { usePages, Pager, StyledPanel } from 'design/DataTable/Paged';
import InputSearch from 'teleport/components/InputSearch';
import * as Labels from 'design/Label';
import cfg from 'teleport/config';

export default function ClustersList(props: Props) {
  const { clusters, search = '', pageSize = 50, onSearchChange } = props;
  const [sorting, setSorting] = React.useState<Sorting>({
    clusterId: 'DESC',
  });

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

  return (
    <>
      <Flex mb={4} alignItems="center" justifyContent="flex-start">
        <InputSearch height="30px" mr="3" onChange={onSearchChange} />
      </Flex>
      <StyledPanel
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
        justifyContent="space-between"
      >
        <Pager {...paged} />
      </StyledPanel>
      <StyledTable data={paged.data}>
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
          header={<Cell />}
          cell={<ActionCell flags={props.menuFlags} />}
        />
      </StyledTable>
    </>
  );
}

function filter(clusters: Cluster[], searchValue = '') {
  return clusters.filter(obj =>
    isMatch(obj, searchValue, {
      searchableProps: ['clusterId'],
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
  return <Cell>{clusterId}</Cell>;
}

function RootLabelCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];
  const isRoot = cfg.proxyCluster === clusterId;
  return <Cell>{isRoot && <Labels.Primary>ROOT</Labels.Primary>}</Cell>;
}

function ActionCell(props: { flags: MenuFlags }) {
  const { rowIndex, data } = props as any;
  const { clusterId } = data[rowIndex];
  const $items = [] as React.ReactNode[];

  if (props.flags.showNodes) {
    $items.push(renderMenuItem('Servers', cfg.getNodesRoute(clusterId)));
  }
  if (props.flags.showApps) {
    $items.push(renderMenuItem('Applications', cfg.getAppsRoute(clusterId)));
  }
  if (props.flags.showKubes) {
    $items.push(
      renderMenuItem('Kubernetes', cfg.getKubernetesRoute(clusterId))
    );
  }
  if (props.flags.showDatabases) {
    $items.push(renderMenuItem('Databases', cfg.getDatabasesRoute(clusterId)));
  }
  if (props.flags.showDesktops) {
    $items.push(
      renderMenuItem('Desktops (preview)', cfg.getDesktopsRoute(clusterId))
    );
  }
  if (props.flags.showAudit) {
    $items.push(renderMenuItem('Audit Log', cfg.getAuditRoute(clusterId)));
  }
  if (props.flags.showRecordings) {
    $items.push(
      renderMenuItem('Session Recordings', cfg.getRecordingsRoute(clusterId))
    );
  }

  return (
    <Cell align="right">{$items && <MenuButton children={$items} />}</Cell>
  );
}

function renderMenuItem(name: string, url: string) {
  return (
    <MenuItem as={NavLink} to={url} key={name}>
      {name}
    </MenuItem>
  );
}

type SortCol = keyof Cluster;

type Sorting = {
  [P in keyof Cluster]?: string;
};

type Props = {
  clusters: Cluster[];
  onSearchChange: (value: string) => void;
  search: string;
  pageSize?: 500;
  menuFlags: MenuFlags;
};

type MenuFlags = {
  showNodes: boolean;
  showAudit: boolean;
  showRecordings: boolean;
  showApps: boolean;
  showDatabases: boolean;
  showKubes: boolean;
  showDesktops: boolean;
};

const StyledTable = styled(Table)`
  td {
    height: 22px;
  }
`;
