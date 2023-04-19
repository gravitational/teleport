/**
 * Copyright 2020 Gravitational, Inc.
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
import { Label } from 'design';
import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { User } from 'teleport/services/user';

export default function UserList({
  users = [],
  pageSize = 20,
  onEdit,
  onDelete,
  onReset,
}: Props) {
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
          render: ({ roles }) => <RolesCell roles={roles} />,
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
          altKey: 'options-btn',
          render: user => (
            <ActionCell
              user={user}
              onEdit={onEdit}
              onReset={onReset}
              onDelete={onDelete}
            />
          ),
        },
      ]}
      emptyText="No Users Found"
      isSearchable
      pagination={{ pageSize }}
    />
  );

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

const ActionCell = ({
  user,
  onEdit,
  onReset,
  onDelete,
}: {
  user: User;
  onEdit: (user: User) => void;
  onReset: (user: User) => void;
  onDelete: (user: User) => void;
}) => {
  if (!user.isLocal) {
    return <Cell align="right" />;
  }

  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={() => onEdit(user)}>Edit...</MenuItem>
        <MenuItem onClick={() => onReset(user)}>
          Reset Authentication...
        </MenuItem>
        <MenuItem onClick={() => onDelete(user)}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};

const RolesCell = ({ roles }: Pick<User, 'roles'>) => {
  const $roles = roles.map(role => (
    <Label mb="1" mr="1" key={role} kind="secondary">
      {role}
    </Label>
  ));

  return <Cell>{$roles}</Cell>;
};

type Props = {
  users: User[];
  pageSize?: number;
  onEdit(user: User): void;
  onDelete(user: User): void;
  onReset(user: User): void;
};
