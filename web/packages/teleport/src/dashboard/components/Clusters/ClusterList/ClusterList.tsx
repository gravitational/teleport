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
import { sortBy } from 'lodash';
import styled from 'styled-components';
import isMatch from 'design/utils/match';
import { Text } from 'design';
import { Lan, Cli } from 'design/Icon';
import { MenuItemIcon } from 'design/Menu';
import { Cluster } from 'teleport/services/clusters';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import {
  SortHeaderCell,
  TextCell,
  Cell,
  Table,
  Column,
  SortTypes,
} from 'design/DataTable';
import { usePages, Pager, StyledPanel } from 'design/DataTable/Paged';
import * as Labels from 'design/Label';
import cfg from 'teleport/config';

export default function ClustersList(props: Props) {
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
          cell={<NodeCountCell />}
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
      </StyledTable>
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

export function NodeCountCell(props) {
  const { rowIndex, data } = props;
  const { nodeCount, clusterId } = data[rowIndex];
  const terminalURL = cfg.getConsoleNodesRoute(clusterId);

  if (!clusterId) {
    return <Cell />;
  }

  // show empty button when 0 nodes
  const btnText = nodeCount > 0 ? nodeCount : '';

  return (
    <Cell>
      <StyledConsoleLink
        title="Open Cluster Terminal"
        target="_blank"
        as="a"
        href={terminalURL}
      >
        <Cli /> {btnText}
      </StyledConsoleLink>
    </Cell>
  );
}

export function NameCell(props) {
  const { rowIndex, data } = props;
  const { clusterId, url } = data[rowIndex];
  return <Cell>{url && <StyledLink to={url}>{clusterId}</StyledLink>}</Cell>;
}

function RootLabelCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];
  const isRoot = cfg.proxyCluster === clusterId;
  return <Cell>{isRoot && <Labels.Primary>ROOT</Labels.Primary>}</Cell>;
}

function ActionCell(props) {
  const { rowIndex, data } = props;
  const { clusterId } = data[rowIndex];

  if (!clusterId) {
    return <Cell />;
  }

  const nodeListURL = cfg.getClusterRoute(clusterId);
  const terminalURL = cfg.getConsoleNodesRoute(clusterId);
  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem as={NavLink} to={nodeListURL}>
          <MenuItemIcon fontSize="2" as={Lan} />
          View Cluster
        </MenuItem>
        <MenuItem as="a" href={terminalURL} target="_blank">
          <MenuItemIcon fontSize="2" as={Cli} />
          Open Terminal
        </MenuItem>
      </MenuButton>
    </Cell>
  );
}

type SortCol = keyof Cluster;

type Sorting = {
  [P in keyof Cluster]?: string;
};

type Props = {
  clusters: Cluster[];
  search: string;
  pageSize?: 500;
};

const StyledTable = styled(Table)`
  td {
    height: 22px;
  }
`;

const StyledLink = styled(Text)(
  props => `
  text-decoration: underline;
  font-weight: ${props.theme.fontWeights.bold};
  &:focus {
    background: #2c3a73;
    padding: 2px 4px;
    margin: 0 -4px;
  }
`
);

const StyledConsoleLink = styled(Text)(
  props => `
  border: 1px solid ${props.theme.colors.dark};
  display: inline-flex;
  align-items: center;
  border-radius: 2px;
  padding: 0 8px 0 2px;
  height: 16px;
  font-size: 10px;
  text-decoration: none;
  background: ${props.theme.colors.bgTerminal};
  color: ${props.theme.colors.light};
  font-weight: ${props.theme.fontWeights.regular};
  transition: all .3s;
  min-width: 24px;

  &:hover, &:focus {
    box-shadow: 0 4px 16px rgba(0, 0, 0, .24);
    border: 1px solid ${props.theme.colors.accent};
  }
  span {
    margin-right: 4px;
    opacity: .32;
  }
`
);

StyledLink.defaultProps = {
  color: 'text.primary',
  typography: 'body2',
  as: NavLink,
};
