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
