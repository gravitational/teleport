/*
Copyright 2021 Gravitational, Inc.

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
import Table, { Cell, LabelCell } from 'design/DataTable';
import { Desktop } from 'teleport/services/desktops';
import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';

function DesktopList(props: Props) {
  const {
    desktops = [],
    pageSize = 100,
    onLoginMenuOpen,
    onLoginSelect,
  } = props;

  function onDesktopSelect(
    e: React.MouseEvent,
    username: string,
    desktopName: string
  ) {
    e.preventDefault();
    onLoginSelect(username, desktopName);
  }

  return (
    <Table
      data={desktops}
      columns={[
        {
          key: 'addr',
          headerText: 'Address',
          isSortable: true,
        },
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'tags',
          headerText: 'Labels',
          render: ({ tags }) => <LabelCell data={tags} />,
        },
        {
          altKey: 'login-cell',
          render: desktop =>
            renderLoginCell(desktop, onLoginMenuOpen, onDesktopSelect),
        },
      ]}
      pagination={{
        pageSize,
      }}
      initialSort={{
        key: 'name',
        dir: 'ASC',
      }}
      isSearchable
      emptyText="No Desktops Found"
    />
  );
}

function renderLoginCell(
  { name }: Desktop,
  onOpen: (serverUuid: string) => LoginItem[],
  onSelect: (
    e: React.SyntheticEvent,
    username: string,
    desktopName: string
  ) => void
) {
  function handleOnOpen() {
    return onOpen(name);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    return onSelect(e, login, name);
  }

  return (
    <Cell align="right">
      <MenuLogin
        getLoginItems={handleOnOpen}
        onSelect={handleOnSelect}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'center',
          horizontal: 'right',
        }}
      />
    </Cell>
  );
}

type Props = {
  desktops: Desktop[];
  pageSize?: number;
  username: string;
  clusterId: string;
  onLoginMenuOpen(desktopName: string): { login: string; url: string }[];
  onLoginSelect(username: string, desktopName: string): void;
};

export default DesktopList;
