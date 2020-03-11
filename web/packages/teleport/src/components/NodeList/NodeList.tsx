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
import { Flex, Label } from 'design';
import { borderRadius } from 'design/system';
import {
  Table,
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
} from 'design/DataTable';
import { usePages } from 'design/DataTable/Paged';
import MenuSshLogin, { LoginItem } from 'shared/components/MenuSshLogin';
import { Node } from 'teleport/services/nodes';
import Pager, { StyledButtons } from 'design/DataTable/Paged/Pager';
import InputSearch from 'teleport/components/InputSearch';

type NodeListProps = {
  nodes: Node[];
  onLoginMenuOpen: (serverId: string) => { login: string; url: string }[];
  onLoginSelect: (login: string, serverId: string) => void;
  pageSize?: number;
};

function NodeList(props: NodeListProps) {
  const { nodes = [], onLoginMenuOpen, onLoginSelect, pageSize = 100 } = props;
  const [searchValue, setSearchValue] = React.useState('');
  const [sortDir, setSortDir] = React.useState<Record<string, string>>({
    hostname: SortTypes.ASC,
  });

  function sortAndFilter(search) {
    const filtered = nodes.filter(obj =>
      isMatch(obj, search, {
        searchableProps: ['hostname', 'addr', 'tags'],
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

  function onSearchChange(value) {
    setSearchValue(value);
  }

  const data = sortAndFilter(searchValue);
  const pagging = usePages({ pageSize, data });

  return (
    <div style={{ width: '100%' }}>
      <StyledPanel
        alignItems="center"
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
        justifyContent="space-between"
      >
        <InputSearch height="30px" mr="3" onChange={onSearchChange} />
        {pagging.hasPages && (
          <Flex alignItems="center" justifyContent="flex-end">
            <Pager {...pagging} />
          </Flex>
        )}
      </StyledPanel>
      <StyledTable data={pagging.data}>
        <Column
          header={<Cell>Session</Cell>}
          cell={<LoginCell onOpen={onLoginMenuOpen} onSelect={onLoginSelect} />}
        />
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
          cell={<TextCell />}
        />
        <Column header={<Cell>Labels</Cell>} cell={<LabelCell />} />
      </StyledTable>
    </div>
  );
}

function searchAndFilterCb(
  targetValue: any[],
  searchValue: string,
  propName: string
) {
  if (propName === 'tags') {
    return targetValue.some(item => {
      const { name, value } = item;
      return (
        name.toLocaleUpperCase().indexOf(searchValue) !== -1 ||
        value.toLocaleUpperCase().indexOf(searchValue) !== -1
      );
    });
  }
}

const LoginCell: React.FC<Required<{
  onSelect: (login: string, serverId: string) => void;
  onOpen: (serverId: string) => LoginItem[];
  [key: string]: any;
}>> = props => {
  const { rowIndex, data, onOpen, onSelect } = props;
  const { hostname, id } = data[rowIndex];
  const serverId = hostname || id;
  function handleOnOpen() {
    return onOpen(serverId);
  }

  function handleOnSelect(login) {
    return onSelect(login, serverId);
  }

  return (
    <Cell>
      <MenuSshLogin onOpen={handleOnOpen} onSelect={handleOnSelect} />
    </Cell>
  );
};

export function LabelCell(props) {
  const { rowIndex, data } = props;
  const { tags } = data[rowIndex];
  const $labels = tags.map(({ name, value }) => (
    <Label mb="1" mr="1" key={name} kind="secondary">
      {`${name}: ${value}`}
    </Label>
  ));

  return <Cell>{$labels}</Cell>;
}

const StyledPanel = styled(Flex)`
  box-sizing: content-box;
  padding: 16px;
  height: 24px;
  background: ${props => props.theme.colors.primary.light};
  ${borderRadius}
  ${StyledButtons} {
    margin-left: ${props => `${props.theme.space[3]}px`};
  }
`;

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: baseline;
  }
`;

export default NodeList;
