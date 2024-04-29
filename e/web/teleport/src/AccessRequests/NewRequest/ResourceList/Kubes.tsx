import React from 'react';
import { ClickableLabelCell } from 'design/DataTable';
import { Kube } from 'teleport/services/kube';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Kubes(props: ListProps & { kubes: Kube[] }) {
  const {
    kubes = [],
    addedResources,
    customSort,
    onLabelClick,
    addOrRemoveResource,
  } = props;

  return (
    <StyledTable
      data={kubes}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
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
              Boolean(addedResources.kube_cluster[agent.name]),
              () => addOrRemoveResource('kube_cluster', agent.name)
            ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}
