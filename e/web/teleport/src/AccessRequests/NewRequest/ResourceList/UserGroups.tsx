import React from 'react';
import { ClickableLabelCell } from 'design/DataTable';
import { UserGroup } from 'teleport/services/userGroups';

import { ListProps, StyledTable, renderActionCell } from './ResourceList';

export function UserGroups(props: ListProps & { userGroups: UserGroup[] }) {
  const {
    userGroups = [],
    addedResources,
    customSort,
    onLabelClick,
    addOrRemoveResource,
  } = props;

  return (
    <StyledTable
      data={userGroups}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
          render: ({ friendlyName, name }) => <td>{friendlyName || name}</td>,
        },
        {
          key: 'description',
          headerText: 'Description',
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
              Boolean(addedResources.user_group[agent.name]),
              () =>
                addOrRemoveResource(
                  'user_group',
                  agent.name,
                  agent.friendlyName
                )
            ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}
