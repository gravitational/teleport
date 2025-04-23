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

import Table, { Cell, LabelCell } from 'design/DataTable';

import { User } from 'teleport/services/user';

import { renderActionCell, SimpleListProps } from '../common';

export default function UserList(props: SimpleListProps & { users: User[] }) {
  const {
    users = [],
    selectedResources,
    toggleSelectResource,
    fetchStatus,
  } = props;

  return (
    <Table
      data={users}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'roles',
          headerText: 'Roles',
          isSortable: true,
          onSort: (a, b) => {
            const aStr = a.roles.toString();
            const bStr = b.roles.toString();

            if (aStr < bStr) {
              return -1;
            }
            if (aStr > bStr) {
              return 1;
            }

            return 0;
          },
          render: ({ roles }) => <LabelCell data={roles} />,
        },
        {
          key: 'authType',
          headerText: 'Type',
          isSortable: true,
          render: ({ authType }) => (
            <Cell style={{ textTransform: 'capitalize' }}>
              {renderAuthType(authType)}
            </Cell>
          ),
        },
        {
          altKey: 'action-btn',
          render: ({ name }) =>
            renderActionCell(Boolean(selectedResources.user[name]), () =>
              toggleSelectResource({ kind: 'user', targetValue: name })
            ),
        },
      ]}
      emptyText="No Users Found"
      isSearchable
      pagination={{ pageSize: props.pageSize }}
      fetching={{
        fetchStatus,
      }}
    />
  );

  // TODO(lisa): do properly
  function renderAuthType(authType: string) {
    switch (authType) {
      case 'github':
        return 'GitHub';
      case 'saml':
        return 'SAML';
      case 'oidc':
        return 'OIDC';
    }
    return authType;
  }
}
