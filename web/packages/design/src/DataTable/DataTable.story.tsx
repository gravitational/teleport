/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { ClickableLabelCell, DateCell, LabelCell } from './Cells';
import Table from './Table';
import { TableProps } from './types';

export default {
  title: 'Design/DataTable',
};

// `if (state.pagination)` is the second view conditionally rendered by Table
// it returns a PagedTable wrapped in StyledTableWrapper
export const WithPagination = () => {
  const props = getDefaultProps();
  props.pagination = {
    pageSize: 7,
    pagerPosition: 'top',
  };
  return <Table<DummyDataType> {...props} />;
};

export const WithPaginationEmpty = () => {
  const props = getDefaultProps();
  props.data = [];
  props.pagination = {
    pageSize: 7,
    pagerPosition: 'top',
  };
  return <Table<DummyDataType> {...props} />;
};

// `if (isSearchable)` is the third view conditionally rendered by Table
// it returns a SearchableBasicTable wrapped in StyledTableWrapper
export const IsSearchable = () => {
  const props = getDefaultProps();
  props.isSearchable = true;
  return <Table<DummyDataType> {...props} />;
};

export const IsSearchableEmpty = () => {
  const props = getDefaultProps();
  props.isSearchable = true;
  props.data = [];
  return <Table<DummyDataType> {...props} />;
};

// the default view rendered by Table
// it returns a BasicTable
export const DefaultBasic = () => {
  const props = getDefaultProps();
  return <Table<DummyDataType> {...props} />;
};

export const DefaultBasicEmpty = () => {
  const props = getDefaultProps();
  props.data = [];
  return <Table<DummyDataType> {...props} />;
};

export const EmptyWithHint = () => {
  const props = getDefaultProps();
  props.data = [];
  props.emptyHint = 'Gimme some data';
  return <Table<DummyDataType> {...props} />;
};

// state.pagination table view with fetching props
export const ClientSidePagination = () => {
  const [allData, setAllData] = useState(data);

  const props = getDefaultProps();
  props.isSearchable = true;
  props.pagination = {
    pageSize: 7,
    pagerPosition: 'top',
  };
  props.fetching = {
    onFetchMore: () => setAllData([...allData, ...extraData]),
    fetchStatus: '',
  };
  props.data = allData;

  return <Table<DummyDataType> {...props} />;
};

// `if (issearchable)` view with ISO date strings
export function ISODateStrings() {
  const props = getDefaultIsoProps();
  props.initialSort = { key: 'createdDate', dir: 'DESC' };
  props.emptyText = 'No Dummy Data Found';
  props.isSearchable = true;
  return <Table<DummyDataISOStringType> {...props} />;
}

// default basic table with interactive cells
export function DefaultAndClickableLabels() {
  return (
    <Table<DefaultAndClickableLabelsDataType>
      columns={[
        { key: 'name', headerText: 'Name', isSortable: true },
        {
          key: 'tags',
          headerText: 'Clickable Labels',
          render: row => (
            <ClickableLabelCell
              labels={row.tags}
              onClick={label => console.log('Label clicked', label)}
            />
          ),
        },
        {
          key: 'tags2',
          headerText: 'Default Labels',
          render: row => <LabelCell data={row.tags2} />,
        },
      ]}
      data={defaultAndClickableData}
      emptyText={'No Dummy Data Found'}
    ></Table>
  );
}

const getDefaultProps = (): TableProps<DummyDataType> => ({
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

const getDefaultIsoProps = (): TableProps<DummyDataISOStringType> => ({
  data: isoDateStringData,
  emptyText: 'No Dummy Data Found',
  columns: [
    { key: 'name', headerText: 'Name', isSortable: true },
    { key: 'desc', headerText: 'Description' },
    { key: 'amount', headerText: 'Amount', isSortable: true },
    {
      key: 'createdDate',
      headerText: 'Created Date',
      isSortable: true,
    },
    {
      key: 'removedDate',
      headerText: 'Removed Date',
      isSortable: true,
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

const data: DummyDataType[] = [
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

const extraData: DummyDataType[] = [
  {
    name: 'f-test',
    desc: 'this is f test',
    amount: 1,
    createdDate: new Date(1636467176000),
    removedDate: new Date(1636423403000),
    tags: ['test9: test9', 'mama: papa', 'test2: test2'],
    bool: true,
  },
  {
    name: 'g-test',
    desc: 'this is g test',
    amount: 55,
    createdDate: new Date(1635367176000),
    removedDate: new Date(1635323403000),
    tags: ['test10: test10', 'mama: papa', 'test4: test4', 'test5: test5'],
    bool: true,
  },
  {
    name: 'h-test',
    desc: 'this is another item',
    amount: 14141,
    createdDate: new Date(1635467176000),
    removedDate: new Date(1635423403000),
    tags: ['test11: test11', 'mama: papa'],
    bool: false,
  },
  {
    name: 'i-test',
    desc: 'yet another item',
    amount: -50,
    createdDate: new Date(1635364176),
    removedDate: new Date(1635322403),
    tags: ['test12: test12'],
    bool: false,
  },
  {
    name: 'j-test',
    desc: 'and another',
    amount: -20,
    createdDate: new Date(1635364176),
    removedDate: new Date(1635322403),
    tags: ['test13: test13'],
    bool: false,
  },
];

const isoDateStringData: DummyDataISOStringType[] = [
  {
    name: 'i-test',
    desc: 'yet another item',
    amount: -50,
    createdDate: '2022-09-09T19:08:17.27Z',
    removedDate: new Date(1635322403).toISOString(),
    tags: ['test12: test12'],
    bool: true,
  },
  {
    name: 'j-test',
    desc: 'and another',
    amount: -20,
    createdDate: '2022-09-09T19:08:17.261Z',
    removedDate: new Date(1635322410).toISOString(),
    tags: ['test13: test13'],
    bool: false,
  },
];

const defaultAndClickableData: DefaultAndClickableLabelsDataType[] = [
  {
    name: 'first',
    tags: [
      {
        name: 'tag1',
        value: 'value1',
      },
      {
        name: 'tag2',
        value: 'value2',
      },
    ],
    tags2: ['t1', 't2', 'some text'],
  },
  {
    name: 'second row with some text and stuff',
    tags: [
      {
        name: 'tag1',
        value: `a bit longer value a bit longer value a bit longer value 
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value`,
      },
      {
        name: 'tag2',
        value: 'value2',
      },
    ],
    tags2: [
      't1',
      't2',
      'some text',
      `a bit longer value a bit longer value a bit longer value 
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value`,
    ],
  },
  {
    name: 'third row with some text and stuff',
    tags: [
      {
        name: 'tag1',
        value: `a bit longer value a bit longer value a bit longer value 
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value`,
      },
      {
        name: 'tag2',
        value: 'value2',
      },
      {
        name: 'tag1',
        value: `a bit longer value a bit longer value a bit longer value 
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value
        a bit longer value a bit longer value`,
      },
    ],
    tags2: [
      't1',
      't2',
      'some text',
      `a bit longer value a bit longer value a bit longer value 
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value`,
      `a bit longer value a bit longer value a bit longer value 
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value
      a bit longer value a bit longer value`,
    ],
  },
];

type DummyDataType = {
  name: string;
  desc: string;
  amount: number;
  createdDate: Date;
  removedDate: Date;
  tags: string[];
  bool: boolean;
};

type DummyDataISOStringType = Omit<
  DummyDataType,
  'createdDate' | 'removedDate'
> & {
  createdDate: string;
  removedDate: string;
};

type DefaultAndClickableLabelsDataType = {
  name: string;
  tags: { name: string; value: string }[];
  tags2: string[];
};
