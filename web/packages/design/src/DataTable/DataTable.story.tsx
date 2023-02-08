/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';

import Table from './Table';
import { LabelCell, DateCell } from './Cells';

export default {
  title: 'Design/DataTable',
};

export const DataTable = () => {
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

export const DataTablePaged = () => {
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

type DummyDataType = {
  name: string;
  desc: string;
  amount: number;
  createdDate: Date;
  removedDate: Date;
  tags: string[];
  bool: boolean;
};
