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

import React, { useState } from 'react';
import styled from 'styled-components';
import { sortBy } from 'lodash';
import {
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
  renderLabelCell,
} from 'design/DataTable';
import Table from 'design/DataTable/Paged';
import isMatch from 'design/utils/match';
import { Desktop } from 'teleport/services/desktops';
import MenuSshLogin, { LoginItem } from 'shared/components/MenuSshLogin';

function DesktopList(props: Props) {
  const {
    desktops = [],
    pageSize = 100,
    search,
    onSearchChange,
    onLoginMenuOpen,
    onLoginSelect,
  } = props;

  const [sortDir, setSortDir] = useState<Record<string, string>>({
    name: SortTypes.DESC,
  });

  function sortAndFilter(search) {
    const filtered = desktops.filter(obj =>
      isMatch(obj, search, {
        searchableProps: ['name', 'addr'],
        cb: searchAndFilterCb,
      })
    );

    const columnKey = Object.getOwnPropertyNames(sortDir)[0];
    const sorted = sortBy(filtered, columnKey);
    if (sortDir[columnKey] === SortTypes.ASC) {
      return sorted.reverse();
    }

    return sorted;
  }

  function onSortChange(columnKey: string, sortDir: string) {
    setSortDir({ [columnKey]: sortDir });
  }

  function onDesktopSelect(
    e: React.MouseEvent,
    username: string,
    desktopName: string
  ) {
    e.preventDefault();
    onLoginSelect(username, desktopName);
  }

  const data = sortAndFilter(search);

  return (
    <StyledTable
      pageSize={pageSize}
      data={data}
      search={search}
      onSearchChange={onSearchChange}
    >
      <Column
        columnKey="addr"
        header={
          <SortHeaderCell
            sortDir={sortDir.addr}
            onSortChange={onSortChange}
            title="Address"
          />
        }
        cell={<AddressCell />}
      />
      <Column
        columnKey="name"
        header={
          <SortHeaderCell
            sortDir={sortDir.name}
            onSortChange={onSortChange}
            title="Name"
          />
        }
        cell={<TextCell />}
      />
      <Column header={<Cell>Labels</Cell>} cell={<LabelCell />} />
      <Column
        header={<Cell />}
        cell={<LoginCell onOpen={onLoginMenuOpen} onSelect={onDesktopSelect} />}
      />
    </StyledTable>
  );
}

const AddressCell = props => {
  // If default RDP port (3389) is present, don't show it
  const { rowIndex, data, columnKey, ...rest } = props;
  const addr = data[rowIndex][columnKey];

  return <Cell {...rest}>{addr}</Cell>;
};

// TODO(isaiah): may be able to be abstracted out from here/NodeList.tsx
const LoginCell: React.FC<
  Required<{
    onSelect?: (
      e: React.SyntheticEvent,
      username: string,
      desktopName: string
    ) => void;
    onOpen: (serverUuid: string) => LoginItem[];
    [key: string]: any;
  }>
> = props => {
  const { rowIndex, data, onOpen, onSelect } = props;
  const { name } = data[rowIndex] as Desktop;
  const desktopName = name;
  function handleOnOpen() {
    return onOpen(desktopName);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    return onSelect(e, login, desktopName);
  }

  return (
    <Cell align="right">
      <MenuSshLogin
        onOpen={handleOnOpen}
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
};

function LabelCell(props) {
  const { rowIndex, data } = props;
  const { tags = [] } = data[rowIndex];
  return renderLabelCell(tags);
}

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: baseline;
  }
`;

function searchAndFilterCb(
  targetValue: any[],
  searchValue: string,
  propName: string
) {
  if (propName === 'tags') {
    return targetValue.some(item => {
      return item.toLocaleUpperCase().indexOf(searchValue) !== -1;
    });
  }
}

type Props = {
  desktops: Desktop[];
  pageSize?: number;
  username: string;
  clusterId: string;
  search: string;
  onSearchChange: React.Dispatch<React.SetStateAction<string>>;
  onLoginMenuOpen(desktopName: string): { login: string; url: string }[];
  onLoginSelect(username: string, desktopName: string): void;
};

export default DesktopList;
