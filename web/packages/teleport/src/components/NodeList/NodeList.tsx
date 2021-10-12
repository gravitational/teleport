/*
Copyright 2019 Gravitational, Inc.

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
import styled from 'styled-components';
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import {
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
  renderLabelCell,
} from 'design/DataTable';
import Table from 'design/DataTable/Paged';
import MenuSshLogin, { LoginItem } from 'shared/components/MenuSshLogin';
import { Node } from 'teleport/services/nodes';

function NodeList(props: Props) {
  const {
    nodes = [],
    searchValue,
    onLoginMenuOpen,
    onLoginSelect,
    pageSize = 100,
  } = props;
  const [sortDir, setSortDir] = React.useState<Record<string, string>>({
    hostname: SortTypes.DESC,
  });

  function sortAndFilter(search) {
    const filtered = nodes.filter(obj =>
      isMatch(obj, search, {
        searchableProps: ['hostname', 'addr', 'tags', 'tunnel'],
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

  const data = sortAndFilter(searchValue);

  return (
    <div>
      <StyledTable pageSize={pageSize} data={data}>
        <Column
          columnKey="hostname"
          header={
            <SortHeaderCell
              sortDir={sortDir.hostname}
              onSortChange={onSortChange}
              title="Hostname"
            />
          }
          cell={<TextCell />}
        />
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
        <Column header={<Cell>Labels</Cell>} cell={<LabelCell />} />
        <Column
          header={<Cell />}
          cell={<LoginCell onOpen={onLoginMenuOpen} onSelect={onLoginSelect} />}
        />
      </StyledTable>
    </div>
  );
}

function searchAndFilterCb(
  targetValue: any[],
  searchValue: string,
  propName: string
) {
  if (propName === 'tunnel') {
    return 'TUNNEL'.indexOf(searchValue) !== -1;
  }

  if (propName === 'tags') {
    return targetValue.some(item => {
      return item.toLocaleUpperCase().indexOf(searchValue) !== -1;
    });
  }
}

const LoginCell: React.FC<Required<{
  onSelect?: (e: React.SyntheticEvent, login: string, serverId: string) => void;
  onOpen: (serverId: string) => LoginItem[];
  [key: string]: any;
}>> = props => {
  const { rowIndex, data, onOpen, onSelect } = props;
  const { id } = data[rowIndex] as Node;
  const serverId = id;
  function handleOnOpen() {
    return onOpen(serverId);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    return onSelect(e, login, serverId);
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

function AddressCell(props) {
  const { rowIndex, data, ...rest } = props;
  const { addr, tunnel } = data[rowIndex] as Node;
  return <Cell {...rest}>{tunnel ? renderTunnel() : addr}</Cell>;
}

function renderTunnel() {
  return (
    <span
      style={{ cursor: 'default' }}
      title="This node is connected to cluster through reverse tunnel"
    >{`⟵ tunnel`}</span>
  );
}

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

type Props = {
  nodes: Node[];
  onLoginMenuOpen(serverId: string): { login: string; url: string }[];
  onLoginSelect(e: React.SyntheticEvent, login: string, serverId: string): void;
  pageSize?: number;
  searchValue: string;
};

export default NodeList;
