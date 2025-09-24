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
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { SearchPanel } from 'shared/components/Search';

import { SeversidePagination } from 'teleport/components/hooks/useServersidePagination';
import { Access, User, UserOrigin } from 'teleport/services/user';

export default function UserList({
  onEdit,
  onDelete,
  onReset,
  onSearchChange,
  search,
  serversidePagination,
  usersAcl,
}: Props) {
  const canEdit = usersAcl.edit;
  const canDelete = usersAcl.remove;

  return (
    <Table
      data={serversidePagination.fetchedData.agents}
      fetching={{
        fetchStatus: serversidePagination.fetchStatus,
        onFetchNext: serversidePagination.fetchNext,
        onFetchPrev: serversidePagination.fetchPrev,
      }}
      serversideProps={{
        sort: undefined,
        setSort: () => undefined,
        serversideSearchPanel: (
          <SearchPanel
            updateSearch={onSearchChange}
            updateQuery={null}
            hideAdvancedSearch={true}
            filter={{ search }}
            disableSearch={serversidePagination.fetchStatus === 'loading'}
          />
        ),
      }}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          key: 'roles',
          headerText: 'Roles',
          render: ({ roles }) => <LabelCell data={roles} />,
        },
        {
          key: 'authType',
          headerText: 'Type',
          render: ({ authType, origin, isBot }) => (
            <Cell style={{ textTransform: 'capitalize' }}>
              {renderAuthType(authType, origin, isBot)}
            </Cell>
          ),
        },
        {
          altKey: 'options-btn',
          render: (user: User) => (
            <ActionCell
              user={user}
              canEdit={canEdit}
              canDelete={canDelete}
              onEdit={() => onEdit(user)}
              onReset={() => onReset(user)}
              onDelete={() => onDelete(user)}
            />
          ),
        },
      ]}
      emptyText="No Users Found"
      isSearchable
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
  canEdit,
  canDelete,
  onEdit,
  onReset,
  onDelete,
}: {
  user: User;
  canEdit: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onReset: () => void;
  onDelete: () => void;
}) => {
  if (!(canEdit || canDelete)) {
    return <Cell align="right" />;
  }

  if (user.isBot || !user.isLocal) {
    return <Cell align="right" />;
  }

  return (
    <Cell align="right">
      <MenuButton>
        {canEdit && <MenuItem onClick={onEdit}>Edit...</MenuItem>}
        {canEdit && (
          <MenuItem onClick={onReset}>Reset Authentication...</MenuItem>
        )}
        {canDelete && <MenuItem onClick={onDelete}>Delete...</MenuItem>}
      </MenuButton>
    </Cell>
  );
};

type Props = {
  onEdit(user: User): void;
  onDelete(user: User): void;
  onReset(user: User): void;
  onSearchChange(search: string): void;
  search: string;
  serversidePagination: SeversidePagination<User>;
  usersAcl: Access;
};
