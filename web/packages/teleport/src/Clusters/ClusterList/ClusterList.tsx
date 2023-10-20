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

import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import Table, { Cell } from 'design/DataTable';
import { Primary } from 'design/Label';

import { Cluster } from 'teleport/services/clusters';
import cfg from 'teleport/config';

export default function ClustersList(props: Props) {
  const { clusters = [], pageSize = 50, menuFlags } = props;

  return (
    <StyledTable
      data={clusters}
      columns={[
        {
          altKey: 'root-label',
          render: renderRootLabelCell,
        },
        {
          key: 'clusterId',
          headerText: 'Name',
          isSortable: true,
        },
        {
          altKey: 'menu-btn',
          render: cluster => renderActionCell(cluster, menuFlags),
        },
      ]}
      emptyText="No Clusters Found"
      isSearchable
      showFirst={clusters =>
        clusters.find(c => c.clusterId === cfg.proxyCluster)
      }
      pagination={{ pageSize }}
    />
  );
}

function renderRootLabelCell({ clusterId }: Cluster) {
  const isRoot = cfg.proxyCluster === clusterId;
  return (
    <Cell style={{ width: '40px' }}>{isRoot && <Primary>ROOT</Primary>}</Cell>
  );
}

function renderActionCell({ clusterId }: Cluster, flags: MenuFlags) {
  const $items = [] as React.ReactNode[];

  if (flags.showNodes) {
    $items.push(renderMenuItem('Servers', cfg.getNodesRoute(clusterId)));
  }
  if (flags.showApps) {
    $items.push(renderMenuItem('Applications', cfg.getAppsRoute(clusterId)));
  }
  if (flags.showKubes) {
    $items.push(
      renderMenuItem('Kubernetes', cfg.getKubernetesRoute(clusterId))
    );
  }
  if (flags.showDatabases) {
    $items.push(renderMenuItem('Databases', cfg.getDatabasesRoute(clusterId)));
  }
  if (flags.showDesktops) {
    $items.push(renderMenuItem('Desktops', cfg.getDesktopsRoute(clusterId)));
  }
  if (flags.showAudit) {
    $items.push(renderMenuItem('Audit Log', cfg.getAuditRoute(clusterId)));
  }
  if (flags.showRecordings) {
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
    <MenuItem as={NavLink} color="text.main" to={url} key={name}>
      {name}
    </MenuItem>
  );
}

type Props = {
  clusters: Cluster[];
  pageSize?: number;
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
` as typeof Table;
