/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
