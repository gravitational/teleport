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

import { Cell, ClickableLabelCell } from 'design/DataTable';

import { Node } from 'teleport/services/nodes';

import { renderActionCell, ServerSideListProps, StyledTable } from '../common';

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
