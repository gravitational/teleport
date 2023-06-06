/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
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
          onSort: (a: string[], b: string[]) => {
            const aStr = a.toString();
            const bStr = b.toString();

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
