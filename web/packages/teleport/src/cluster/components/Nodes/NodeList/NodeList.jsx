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
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import {
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
} from 'design/DataTable';
import { Label } from 'design';
import Table from './Table';
import MenuLogin from './../MenuLogin';

function NodeList({ nodes = [], logins, search = '', pageSize = 100 }) {
  const [sortDir, setSortDir] = React.useState({
    hostname: SortTypes.ASC,
  });

  function sortAndFilter() {
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

  function onSortChange(columnKey, sortDir) {
    setSortDir({ [columnKey]: sortDir });
  }

  const data = sortAndFilter();

  return (
    <Table data={data} pageSize={pageSize}>
      <Column
        header={<Cell>Session</Cell>}
        cell={<LoginCell logins={logins} />}
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
    </Table>
  );
}

function searchAndFilterCb(targetValue, searchValue, propName) {
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

const LoginCell = ({ rowIndex, data, logins }) => {
  const { id, hostname } = data[rowIndex];
  return (
    <Cell>
      <MenuLogin serverId={hostname || id} logins={logins} />
    </Cell>
  );
};

export function LabelCell({ rowIndex, data }) {
  const { tags } = data[rowIndex];
  const $labels = tags.map(({ name, value }) => (
    <Label mb="1" mr="1" key={name} kind="secondary">
      {`${name}: ${value}`}
    </Label>
  ));

  return <Cell>{$labels}</Cell>;
}

export default NodeList;
