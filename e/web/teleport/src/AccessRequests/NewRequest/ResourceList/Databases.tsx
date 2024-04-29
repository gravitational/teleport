import React from 'react';
import { ClickableLabelCell } from 'design/DataTable';
import { Database } from 'teleport/services/databases';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Databases(props: ListProps & { databases: Database[] }) {
  const {
    databases = [],
    onLabelClick,
    addedResources,
    addOrRemoveResource,
    customSort,
  } = props;

  return (
    <StyledTable
      data={databases}
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
          key: 'type',
          headerText: 'Type',
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
            renderActionCell(Boolean(addedResources.db[agent.name]), () =>
              addOrRemoveResource('db', agent.name)
            ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}
