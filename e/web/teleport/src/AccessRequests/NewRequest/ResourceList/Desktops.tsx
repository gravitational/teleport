import React from 'react';
import { ClickableLabelCell } from 'design/DataTable';
import { Desktop } from 'teleport/services/desktops';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Desktops(props: ListProps & { desktops: Desktop[] }) {
  const {
    desktops = [],
    addedResources,
    customSort,
    onLabelClick,
    addOrRemoveResource,
  } = props;

  return (
    <StyledTable
      data={desktops}
      columns={[
        {
          key: 'addr',
          headerText: 'Address',
        },
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
              Boolean(addedResources.windows_desktop[agent.name]),
              () => addOrRemoveResource('windows_desktop', agent.name)
            ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}
