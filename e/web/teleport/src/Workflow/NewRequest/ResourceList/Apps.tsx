import React from 'react';
import { Cell, ClickableLabelCell } from 'design/DataTable';
import { App } from 'teleport/services/apps';
import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Apps(props: ListProps & { apps: App[] }) {
  const {
    apps = [],
    addedResources,
    customSort,
    onLabelClick,
    addOrRemoveResource,
  } = props;
  return (
    <StyledTable
      data={apps}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'description',
          headerText: 'Description',
          isSortable: true,
        },
        {
          key: 'publicAddr',
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
            renderActionCell(Boolean(addedResources.app[agent.name]), () =>
              addOrRemoveResource('app', agent.name)
            ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}

function renderAddressCell({ publicAddr }: App) {
  return <Cell>https://{publicAddr}</Cell>;
}
