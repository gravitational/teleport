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

import React, { useState } from 'react';

import Table from './Table';
import { LabelCell, DateCell, ClickableLabelCell } from './Cells';

export default {
  title: 'Design/DataTable',
};

export const VariousColumns = () => {
  return (
    <Table<DummyDataType>
      columns={[
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
          onSort: sortTagsByLength,
        },
        { key: 'bool', headerText: 'Boolean', isSortable: true },
      ]}
      data={data}
      emptyText={'No Dummy Data Found'}
      isSearchable
    />
  );
};

export const ClientSidePagination = () => {
  const [allData, setAllData] = useState(data);

  return (
    <Table<DummyDataType>
      columns={[
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
          onSort: sortTagsByLength,
        },
        { key: 'bool', headerText: 'Boolean', isSortable: true },
      ]}
      pagination={{
        pageSize: 7,
        pagerPosition: 'top',
      }}
      fetching={{
        onFetchMore: () => setAllData([...allData, ...extraData]),
        fetchStatus: '',
      }}
      data={allData}
      emptyText={'No Dummy Data Found'}
      isSearchable
    />
  );
};

export function ISODateStrings() {
  return (
    <Table<DummyDataISOStringType>
      columns={[
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
          onSort: sortTagsByLength,
        },
        { key: 'bool', headerText: 'Boolean', isSortable: true },
      ]}
      initialSort={{ key: 'createdDate', dir: 'DESC' }}
      data={isoDateStringData}
      emptyText={'No Dummy Data Found'}
      isSearchable
    />
  );
}

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

function sortTagsByLength(a: DummyDataType['tags'], b: DummyDataType['tags']) {
  return a.length - b.length;
}

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
    bool: false,
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
