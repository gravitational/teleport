/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useState } from 'react';

import Table, { DateCell, LabelCell, StyledPanel } from 'design/DataTable';
import { TableProps } from 'design/DataTable/types';

import { FilterMap, ListFilters } from './ListFilters';
import { applyFilters } from './utilities';

export default {
  title: 'Shared/ListFilters',
};

type IntegrationTag = 'test1: test1' | 'test2: test2' | 'test3: test3';

type Item = {
  name: string;
  desc: string;
  amount: number;
  createdDate: Date;
  removedDate: Date;
  tags: string[];
  bool: boolean;
};

type Filters = {
  Type: IntegrationTag;
  Names: string;
};

export function ListFiltersBasic() {
  const [filters, setFilters] = useState<FilterMap<Item, Filters>>({
    Names: {
      options: [
        { label: 'A Test', value: 'a-test' },
        { label: 'C Test', value: 'c-test' },
      ],
      selected: ['c-test'],
      apply: (l, s) => l.filter(i => s.includes(i.name)),
    },
    Type: {
      options: [
        { label: 'Test1', value: 'test1: test1' },
        { label: 'Test2', value: 'test2: test2' },
      ],
      selected: [],
      apply: (l, s) => l.filter(i => s.some(tag => i.tags.includes(tag))),
    },
  });

  const tableProps = getDefaultProps();
  tableProps.data = applyFilters(tableProps.data, filters);

  return (
    <>
      <StyledPanel>
        <ListFilters filters={filters} onFilterChange={setFilters} />
      </StyledPanel>
      <Table {...tableProps} />
    </>
  );
}

const data: Item[] = [
  {
    name: 'a-test',
    desc: 'this is a test',
    amount: 1,
    createdDate: new Date(1636467176000),
    removedDate: new Date(1636423403000),
    tags: ['test1: test1', 'mama: papa', 'test2: test2'],
    bool: true,
  },
  {
    name: 'b-test',
    desc: 'this is b test',
    amount: 55,
    createdDate: new Date(1635367176000),
    removedDate: new Date(1635323403000),
    tags: ['test3: test3', 'mama: papa', 'test4: test4', 'test5: test5'],
    bool: true,
  },
  {
    name: 'd-test',
    desc: 'this is another item',
    amount: 14141,
    createdDate: new Date(1635467176000),
    removedDate: new Date(1635423403000),
    tags: ['test6: test6', 'mama: papa'],
    bool: false,
  },
  {
    name: 'c-test',
    desc: 'yet another item',
    amount: -50,
    createdDate: new Date(1635364176),
    removedDate: new Date(1635322403),
    tags: ['test7: test7'],
    bool: false,
  },
  {
    name: 'e-test',
    desc: 'and another',
    amount: -20,
    createdDate: new Date(1635364176),
    removedDate: new Date(1635322403),
    tags: ['test8: test8'],
    bool: true,
  },
  {
    name: 'looong name with something',
    desc: 'looong description with something important in it and stuff......',
    amount: 1,
    createdDate: new Date(1636467176000),
    removedDate: new Date(1636423403000),
    tags: [
      'test3: test3',
      'mama: papa',
      'test4: test4',
      'test5: test5',
      'test6: test6',
      'test7: longer text than normal',
      'test8: test8',
      'test9: test9',
      'test10: test10',
      'test11: test11',
      `some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......
       some relly long text about something or whatever......`,
    ],
    bool: true,
  },
];

const getDefaultProps = (): TableProps<Item> => ({
  data: data,
  emptyText: 'No Dummy Data Found',
  columns: [
    { key: 'name', headerText: 'Name', isSortable: true },
    { key: 'desc', headerText: 'Description' },
    { key: 'amount', headerText: 'Amount', isSortable: true },
    {
      key: 'createdDate',
      headerText: 'Created Date',
      isSortable: true,
      render: row => <DateCell data={row.createdDate} />,
    },
    {
      key: 'removedDate',
      headerText: 'Removed Date',
      isSortable: true,
      render: row => <DateCell data={row.removedDate} />,
    },
    {
      key: 'tags',
      headerText: 'Labels',
      render: row => <LabelCell data={row.tags} />,
      isSortable: true,
      onSort: (a, b) => a.tags.length - b.tags.length,
    },
    { key: 'bool', headerText: 'Boolean', isSortable: true },
  ],
});
