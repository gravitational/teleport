import React from 'react';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function Roles(props: ListProps & { roles: string[] }) {
  const { roles = [], addedResources, addOrRemoveResource } = props;

  return (
    <StyledTable
      data={roles.map(role => ({ role }))}
      pagination={{ pagerPosition: 'top', pageSize: 10 }}
      isSearchable={true}
      columns={[
        {
          key: 'role',
          headerText: 'Role Name',
          isSortable: true,
        },
        {
          altKey: 'action-btn',
          render: ({ role }) =>
            renderActionCell(Boolean(addedResources.role[role]), () =>
              addOrRemoveResource('role', role)
            ),
        },
      ]}
      emptyText="No Requestable Roles Found"
    />
  );
}
