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

import { Cell, LabelCell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { ClientSearcheableTableWithQueryParamSupport } from 'teleport/components/ClientSearcheableTableWithQueryParamSupport';
import { Access, User, UserOrigin } from 'teleport/services/user';

export default function UserList({
  users = [],
  pageSize = 20,
  onEdit,
  onDelete,
  onReset,
  usersAcl,
}: Props) {
  return (
    <ClientSearcheableTableWithQueryParamSupport
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
          render: ({ authType, origin, isBot }) => (
            <Cell style={{ textTransform: 'capitalize' }}>
              {renderAuthType(authType, origin, isBot)}
            </Cell>
          ),
        },
        {
          altKey: 'options-btn',
          render: user => (
            <ActionCell
              acl={usersAcl}
              user={user}
              onEdit={onEdit}
              onReset={onReset}
              onDelete={onDelete}
            />
          ),
        },
      ]}
      emptyText="No Users Found"
      pagination={{ pageSize }}
    />
  );

  function renderAuthType(
    authType: string,
    origin: UserOrigin,
    isBot?: boolean
  ) {
    if (isBot) {
      return 'Bot';
    }

    switch (authType) {
      case 'github':
        return 'GitHub';
      case 'saml':
        switch (origin) {
          case 'okta':
            return 'Okta';
          case 'scim':
            return 'SCIM';
          default:
            return 'SAML';
        }
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
  acl,
}: {
  user: User;
  onEdit: (user: User) => void;
  onReset: (user: User) => void;
  onDelete: (user: User) => void;
  acl: Access;
}) => {
  const canEdit = acl.edit;
  const canDelete = acl.remove;

  if (!(canEdit || canDelete)) {
    return <Cell align="right" />;
  }

  if (user.isBot || !user.isLocal) {
    return <Cell align="right" />;
  }

  return (
    <Cell align="right">
      <MenuButton>
        {canEdit && <MenuItem onClick={() => onEdit(user)}>Edit...</MenuItem>}
        {canEdit && (
          <MenuItem onClick={() => onReset(user)}>
            Reset Authentication...
          </MenuItem>
        )}
        {canDelete && (
          <MenuItem onClick={() => onDelete(user)}>Delete...</MenuItem>
        )}
      </MenuButton>
    </Cell>
  );
};

type Props = {
  users: User[];
  pageSize?: number;
  onEdit(user: User): void;
  onDelete(user: User): void;
  onReset(user: User): void;
  // determines if the viewer is able to edit/delete users. This is used
  // to conditionally render the edit/delete buttons in the ActionCell
  usersAcl: Access;
};
