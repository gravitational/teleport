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

import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { SearchPanel } from 'shared/components/Search';

import { SeversidePagination } from 'teleport/components/hooks/useServersidePagination';
import { RoleResource } from 'teleport/services/resources';
import { Access } from 'teleport/services/user';

export function RoleList({
  onEdit,
  onDelete,
  onSearchChange,
  search,
  serversidePagination,
  rolesAcl,
}: {
  onEdit(id: string): void;
  onDelete(id: string): void;
  onSearchChange(search: string): void;
  search: string;
  serversidePagination: SeversidePagination<RoleResource>;
  rolesAcl: Access;
}) {
  const canView = rolesAcl.list && rolesAcl.read;
  const canEdit = rolesAcl.edit;
  const canDelete = rolesAcl.remove;

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
            disableSearch={serversidePagination.attempt.status === 'processing'}
          />
        ),
      }}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          altKey: 'options-btn',
          render: (role: RoleResource) => (
            <ActionCell
              canView={canView}
              canDelete={canDelete}
              canEdit={canEdit}
              onEdit={() => onEdit(role.id)}
              onDelete={() => onDelete(role.id)}
            />
          ),
        },
      ]}
      emptyText="No Roles Found"
      isSearchable
    />
  );
}

const ActionCell = (props: {
  canView: boolean;
  canEdit: boolean;
  canDelete: boolean;
  onEdit(): void;
  onDelete(): void;
}) => {
  if (!(props.canView || props.canDelete)) {
    return <Cell align="right" />;
  }
  return (
    <Cell align="right">
      <MenuButton>
        {props.canView && (
          <MenuItem onClick={props.onEdit}>
            {props.canEdit ? 'Edit' : 'View Details'}
          </MenuItem>
        )}
        {props.canDelete && (
          <MenuItem onClick={props.onDelete}>Delete</MenuItem>
        )}
      </MenuButton>
    </Cell>
  );
};
