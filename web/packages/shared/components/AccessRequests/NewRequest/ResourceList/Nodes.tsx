import React from 'react';
import { Cell, ClickableLabelCell } from 'design/DataTable';
import { Node } from 'teleport/services/nodes';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Nodes(props: ListProps & { nodes: Node[] }) {
  const {
    nodes = [],
    addedResources,
    customSort,
    onLabelClick,
    addOrRemoveResource,
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
            renderActionCell(Boolean(addedResources.node[agent.id]), () =>
              addOrRemoveResource('node', agent.id, agent.hostname)
            ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}

export const renderAddressCell = ({ addr, tunnel }: Node) => (
  <Cell>{tunnel ? renderTunnel() : addr}</Cell>
);

function renderTunnel() {
  return (
    <span
      style={{ cursor: 'default', whiteSpace: 'nowrap' }}
      title="This node is connected to cluster through reverse tunnel"
    >
      ← tunnel
    </span>
  );
}
