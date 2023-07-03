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

import React from 'react';
import { Cell, ClickableLabelCell } from 'design/DataTable';

import { Node } from 'teleport/services/nodes';

import { ServerSideListProps, StyledTable, renderActionCell } from '../common';

export function Nodes(props: ServerSideListProps & { nodes: Node[] }) {
  const {
    nodes = [],
    selectedResources,
    customSort,
    onLabelClick,
    toggleSelectResource,
  } = props;

  return (
    <StyledTable
      data={nodes}
      columns={[
        {
          key: 'hostname',
          headerText: 'Hostname',
          isSortable: true,
        },
        {
          key: 'addr',
          headerText: 'Address',
          render: renderAddressCell,
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => (
            <ClickableLabelCell labels={labels} onClick={onLabelClick} />
          ),
        },
        {
          altKey: 'action-btn',
          render: agent =>
            renderActionCell(
              Boolean(selectedResources.server_id[agent.id]),
              () =>
                toggleSelectResource({
                  kind: 'server_id',
                  targetValue: agent.id,
                  friendlyName: agent.hostname,
                })
            ),
        },
      ]}
      emptyText="No Nodes Found"
      customSort={customSort}
      disableFilter
      fetching={{
        fetchStatus: props.fetchStatus,
      }}
    />
  );
}

export const renderAddressCell = ({ addr, tunnel }: Node) => (
  <Cell>{tunnel ? renderTunnel() : addr}</Cell>
);

function renderTunnel() {
  return (
    <span
      style={{ cursor: 'default' }}
      title="This node is connected to cluster through reverse tunnel"
    >{`⟵ tunnel`}</span>
  );
}
