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

import React, { useState } from 'react';
import styled from 'styled-components';
import { sortBy } from 'lodash';
import { Flex, Label } from 'design';
import {
  Cell,
  Column,
  TextCell,
  SortHeaderCell,
  SortTypes,
} from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import isMatch from 'design/utils/match';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import InputSearch from 'teleport/components/InputSearch';
import { User } from 'teleport/services/user';

export default function UserList({
  users,
  pageSize,
  onEdit,
  onDelete,
  onReset,
  canDelete,
  canUpdate,
}: Props) {
  const [searchValue, setSearchValue] = useState('');
  const [sort, setSort] = useState<Record<string, string>>({
    key: 'name',
    dir: SortTypes.ASC,
  });

  function onSortChange(key: string, dir: string) {
    setSort({ key, dir });
  }

  function onSearchChange(value: string) {
    setSearchValue(value);
  }

  function sortAndFilter(searchValue: string) {
    const searchableProps = ['name', 'roles'];
    const filtered = users.filter(user =>
      isMatch(user, searchValue, { searchableProps, cb: null })
    );

    // Apply sorting to filtered list.
    const sorted = sortBy(filtered, sort.key);
    if (sort.dir === SortTypes.DESC) {
      return sorted.reverse();
    }

    return sorted;
  }

  const data = sortAndFilter(searchValue);
  const tableProps = { pageSize, data };

  return (
    <>
      <Flex mb={4}>
        <InputSearch height="30px" mr="3" onChange={onSearchChange} />
      </Flex>
      <StyledTable {...tableProps}>
        <Column
          columnKey="name"
          cell={<TextCell />}
          header={
            <SortHeaderCell
              sortDir={sort.key === 'name' ? sort.dir : null}
              onSortChange={onSortChange}
              title="Username"
            />
          }
        />
        <Column
          columnKey="roles"
          cell={<RolesCell />}
          header={
            <SortHeaderCell
              sortDir={sort.key === 'roles' ? sort.dir : null}
              onSortChange={onSortChange}
              title="Roles"
            />
          }
        />
        <Column
          columnKey="authType"
          cell={<TextCell style={{ textTransform: 'capitalize' }} />}
          header={
            <SortHeaderCell
              sortDir={sort.key === 'authType' ? sort.dir : null}
              onSortChange={onSortChange}
              title="Type"
            />
          }
        />
        <Column
          header={<Cell />}
          cell={
            <ActionCell
              canUpdate={canUpdate}
              canDelete={canDelete}
              onEdit={onEdit}
              onDelete={onDelete}
              onResetPassword={onReset}
            />
          }
        />
      </StyledTable>
    </>
  );
}

const ActionCell = props => {
  const {
    rowIndex,
    data,
    onEdit,
    onResetPassword,
    canDelete,
    canUpdate,
    onDelete,
  } = props;

  const user: User = data[rowIndex];

  if ((!canDelete && !canUpdate) || !user.isLocal) {
    return <Cell align="right" />;
  }

  return (
    <Cell align="right">
      <MenuButton>
        {canUpdate && <MenuItem onClick={() => onEdit(user)}>Edit...</MenuItem>}
        {canUpdate && (
          <MenuItem onClick={() => onResetPassword(user)}>
            Reset Password...
          </MenuItem>
        )}
        {canDelete && (
          <MenuItem onClick={() => onDelete(user)}>Delete...</MenuItem>
        )}
      </MenuButton>
    </Cell>
  );
};

const RolesCell = props => {
  const { rowIndex, data } = props;
  const { roles } = data[rowIndex];
  const $roles = roles.sort().map(role => (
    <Label mb="1" mr="1" key={role} kind="secondary">
      {role}
    </Label>
  ));

  return <Cell>{$roles}</Cell>;
};

type Props = {
  users: User[];
  pageSize: number;
  canDelete: boolean;
  canUpdate: boolean;
  onEdit(user: User): void;
  onDelete(user: User): void;
  onReset(user: User): void;
};

const StyledTable = styled(PagedTable)`
  & > tbody > tr > td {
    vertical-align: baseline;
  }
`;
