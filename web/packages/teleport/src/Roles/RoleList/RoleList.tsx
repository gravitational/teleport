/*
Copyright 2019-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { SearchPanel } from 'shared/components/Search';

import { SeversidePagination } from 'teleport/components/hooks/useServersidePagination';
import { RoleResource } from 'teleport/services/resources';

export function RoleList({
  onEdit,
  onDelete,
  onSearchChange,
  search,
  serversidePagination,
}: {
  onEdit(id: string): void;
  onDelete(id: string): void;
  onSearchChange(search: string): void;
  search: string;
  serversidePagination: SeversidePagination<RoleResource>;
}) {
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
            showSearchBar={true}
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

const ActionCell = (props: { onEdit(): void; onDelete(): void }) => {
  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={props.onEdit}>Edit...</MenuItem>
        <MenuItem onClick={props.onDelete}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};
