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
    searchValue,
    onLoginMenuOpen,
    onLoginSelect,
  } = props;

  const [sortDir, setSortDir] = useState<Record<string, string>>({
    name: SortTypes.DESC,
  });

  function sortAndFilter(search) {
    const filtered = desktops.filter(obj =>
      isMatch(obj, search, {
        searchableProps: ['os', 'addr'],
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
    desktopId: string
  ) {
    e.preventDefault();
    onLoginSelect(username, desktopId);
  }

  const data = sortAndFilter(searchValue);

  return (
    <StyledTable pageSize={pageSize} data={data}>
      <Column
        columnKey="addr"
        header={
          <SortHeaderCell
            sortDir={sortDir.name}
            onSortChange={onSortChange}
            title="Hostname"
          />
        }
        cell={<DesktopDomainCell />}
      />
      <Column
        columnKey="os"
        header={
          <SortHeaderCell
            sortDir={sortDir.desc}
            onSortChange={onSortChange}
            title="Operating System"
          />
        }
        cell={<OSCell />}
      />
      <Column header={<Cell>Labels</Cell>} cell={<LabelCell />} />
      <Column
        header={<Cell />}
        cell={<LoginCell onOpen={onLoginMenuOpen} onSelect={onDesktopSelect} />}
      />
    </StyledTable>
  );
}

// Strips default rdp port from an ip address since this unimportant to display
export const stripRdpPort = (addr: string): string => {
  const splitAddr = addr.split(':');
  if (splitAddr.length > 1 && splitAddr[1] === '3389') {
    return splitAddr[0];
  }
  return addr;
};

const DesktopDomainCell = props => {
  // If default RDP port (3389) is present, don't show it
  const { rowIndex, data, columnKey, ...rest } = props;
  const addr = stripRdpPort(data[rowIndex][columnKey]);

  return <Cell {...rest}>{addr}</Cell>;
};

// TODO(isaiah): may be able to be abstracted out from here/NodeList.tsx
const LoginCell: React.FC<Required<{
  onSelect?: (
    e: React.SyntheticEvent,
    username: string,
    desktopId: string
  ) => void;
  onOpen: (serverUuid: string) => LoginItem[];
  [key: string]: any;
}>> = props => {
  const { rowIndex, data, onOpen, onSelect } = props;
  const { name } = data[rowIndex] as Desktop;
  const desktopId = name;
  function handleOnOpen() {
    return onOpen(desktopId);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    const username = login;

    return onSelect(e, username, desktopId);
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

const OSCell = props => {
  const { rowIndex, data, columnKey, ...rest } = props;
  var osText =
    data[rowIndex][columnKey] === 'windows' ? 'Windows' : 'Unknown OS';
  return <Cell {...rest}>{osText}</Cell>;
};

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
  searchValue: string;
  onLoginMenuOpen(desktopId: string): { login: string; url: string }[];
  onLoginSelect(username: string, desktopId: string): void;
};

export default DesktopList;
