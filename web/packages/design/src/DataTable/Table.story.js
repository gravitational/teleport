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
import match from './../utils/match';
import {
  Table,
  TablePaged,
  Column,
  TextCell,
  SortHeaderCell,
  SortTypes,
  EmptyIndicator,
} from './index';

export default {
  title: 'Design/DataTable',
};

export const DataTable = () => (
  <TableSample TableComponent={Table} data={data} />
);

export const Empty = () => <TableSample TableComponent={Table} data={[]} />;

export const NothingFound = () => (
  <TableSample TableComponent={Table} data={data} filter="no_results" />
);

export const PagedTable = () => (
  <TableSample
    TableComponent={TablePaged}
    tableProps={{ pageSize: 3 }}
    data={data}
  />
);

class TableSample extends React.Component {
  searchableProps = ['addr', 'hostname', 'tags'];

  constructor(props) {
    super(props);
    this.state = {
      filter: props.filter || '',
      colSortDirs: {
        hostname: SortTypes.DESC,
      },
    };
  }

  onSortChange = (columnKey, sortDir) => {
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  };

  onFilterChange = value => {
    this.state.filter = value;
    this.setState(this.state);
  };

  searchAndFilterCb(targetValue, searchValue, propName) {
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

  sortAndFilter(data) {
    const { colSortDirs } = this.state;
    const filtered = data.filter(obj =>
      match(obj, this.state.filter, {
        searchableProps: this.searchableProps,
        cb: this.searchAndFilterCb,
      })
    );

    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    let sorted = sortBy(filtered, columnKey);
    if (sortDir === SortTypes.ASC) {
      sorted = sorted.reverse();
    }

    return sorted;
  }

  render() {
    let { data, TableComponent, tableProps } = this.props;
    data = this.sortAndFilter(data);
    const nothingFound = data.length === 0 && this.state.filter.length > 0;

    if (nothingFound) {
      return (
        <EmptyIndicator title='No Results Found for "X458AAZ"'>
          For tips on getting better search results, please read{' '}
          <a href="https://gravitational.com/teleport/docs">
            our documentation
          </a>
        </EmptyIndicator>
      );
    }

    const props = {
      data: data,
      ...tableProps,
    };

    return (
      <TableComponent {...props}>
        <Column
          columnKey="hostname"
          header={
            <SortHeaderCell
              sortDir={this.state.colSortDirs.hostname}
              onSortChange={this.onSortChange}
              title="Hostname"
            />
          }
          cell={<TextCell />}
        />
        <Column
          columnKey="addr"
          header={
            <SortHeaderCell
              sortDir={this.state.colSortDirs.addr}
              onSortChange={this.onSortChange}
              title="Address"
            />
          }
          cell={<TextCell />}
        />
      </TableComponent>
    );
  }
}

const data = [
  {
    hostname: <strong>host-a</strong>,
    addr: '192.168.7.1',
  },
  {
    hostname: <strong>host-b</strong>,
    addr: '192.168.7.2',
  },
  {
    hostname: <strong>host-c</strong>,
    addr: '192.168.7.3',
  },
  {
    hostname: <strong>host-d</strong>,
    addr: '192.168.7.4',
  },
  {
    hostname: <strong>host-3</strong>,
    addr: '192.168.7.4',
  },
];

export { TableSample, data };
