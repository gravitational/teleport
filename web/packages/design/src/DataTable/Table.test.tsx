/**
 * Copyright 2020-2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { render, fireEvent, screen } from 'design/utils/testing';

import Table from './Table';
import { SortIndicator } from './Cells';

const colHeaderKeys = ['hostname', 'addr'] as 'hostname'[] | 'addr'[];
const colHeaders = ['Hostname', 'Address'];
const data = [
  {
    hostname: 'host-a',
    addr: '192.168.7.1',
  },
  {
    hostname: 'host-b',
    addr: '192.168.7.2',
  },
  {
    hostname: 'host-c',
    addr: '192.168.7.3',
  },
  {
    hostname: 'host-d',
    addr: '192.168.7.4',
  },
  {
    hostname: 'host-3',
    addr: '192.168.7.4',
  },
];

describe('design/Table Simple', () => {
  let container = null;

  beforeEach(() => {
    ({ container } = render(
      <Table
        data={data}
        columns={[
          {
            key: colHeaderKeys[0],
            headerText: colHeaders[0],
          },
          {
            key: colHeaderKeys[1],
            headerText: colHeaders[1],
          },
        ]}
        emptyText="No Servers Found"
      />
    ));
  });

  test('there is one table element', () => {
    expect(container.querySelectorAll('table')).toHaveLength(1);
  });

  test('there is one thead element', () => {
    expect(container.querySelectorAll('thead')).toHaveLength(1);
  });

  test('there is one tbody element', () => {
    expect(container.querySelectorAll('tbody')).toHaveLength(1);
  });

  test('number of th tags == number of headers', () => {
    expect(container.querySelectorAll('th')).toHaveLength(colHeaderKeys.length);
  });

  test('each th tag text == header data', () => {
    container.querySelectorAll('th').forEach((thElement, index) => {
      expect(thElement.textContent).toEqual(colHeaders[index]);
    });
  });

  test('number of tr tags in body == data.length', () => {
    expect(
      container.querySelector('tbody').querySelectorAll('tr')
    ).toHaveLength(data.length);
  });

  test('each td tag text == data texts', () => {
    container
      .querySelector('tbody')
      .querySelectorAll('tr')
      .forEach((trElement, index) => {
        expect(trElement.children[0].textContent).toEqual(
          data[index][colHeaderKeys[0]]
        );
        expect(trElement.children[1].textContent).toEqual(
          data[index][colHeaderKeys[1]]
        );
      });
  });
});

describe('design/Table SortIndicator', () => {
  test('sort indicator defaults to sort vertical (neither ASC or DESC)', () => {
    const { container } = render(<SortIndicator />);
    expect(
      container
        .querySelector('span')
        .classList.contains('icon-chevrons-expand-vertical')
    ).toBe(true);
  });

  test('sort indicator respects sortDir prop set to ASC', () => {
    const { container } = render(<SortIndicator sortDir={'ASC'} />);
    expect(
      container.querySelector('span').classList.contains('icon-chevron-up')
    ).toBe(true);
  });

  test('sort indicator respects sortDir prop set to DESC', () => {
    const { container } = render(<SortIndicator sortDir={'DESC'} />);
    expect(
      container.querySelector('span').classList.contains('icon-chevron-down')
    ).toBe(true);
  });

  test('clicking on col headers changes direction', () => {
    const { container } = render(
      <Table
        data={data}
        columns={[
          {
            key: colHeaderKeys[0],
            headerText: colHeaders[0],
            isSortable: true,
          },
          {
            key: colHeaderKeys[1],
            headerText: colHeaders[1],
            isSortable: true,
          },
        ]}
        emptyText="No Servers Found"
      />
    );

    const anchorTags = container.querySelectorAll('a');
    const header1 = anchorTags[0];
    const header2 = anchorTags[1];

    // Table initially sorts with "Hostname" ASC
    expect(
      header1.querySelector('span').classList.contains('icon-chevron-up')
    ).toBe(true);

    // b/c Table is initially sorted by "Hostname"
    // "Address" header starts with sort vertical (neither ASC or DESC)
    // on sort vertical, DESC is default
    fireEvent.click(screen.getByText(colHeaders[1]));
    expect(
      header2.querySelector('span').classList.contains('icon-chevron-down')
    ).toBe(true);

    fireEvent.click(screen.getByText(colHeaders[1]));
    expect(
      header2.querySelector('span').classList.contains('icon-chevron-up')
    ).toBe(true);
  });
});

test('"onSort" prop is respected', () => {
  const dummyFunc = jest.fn(() => -1);

  render(
    <Table
      data={data}
      columns={[
        {
          key: colHeaderKeys[0],
          headerText: colHeaders[0],
          onSort: dummyFunc,
          isSortable: true,
        },
      ]}
      emptyText="No Servers Found"
    />
  );

  expect(dummyFunc).toHaveReturnedWith(-1);
});

test('respects emptyText prop', () => {
  const targetText = 'No Servers Found';
  const { getByText } = render(
    <Table data={[]} columns={[]} emptyText={targetText} />
  );
  const target = getByText(targetText);

  expect(target.textContent).toEqual(targetText);
});

describe('sorting by field defined in key and altSortKey', () => {
  const sample = [
    {
      hostname: 'host-a',
      created: new Date('2022-07-15T15:34:33.256697813Z'),
      durationText: '1 hour',
    },
    {
      hostname: 'host-b',
      created: new Date('2022-07-05T15:34:33.256697813Z'),
      durationText: '1 second',
    },
    {
      hostname: 'host-c',
      created: new Date('2022-07-10T15:34:33.256697813Z'),
      durationText: '1 minute',
    },
  ];

  // Sorted by string ASC.
  const expectedSortedByKey = ['1 hour', '1 minute', '1 second'];
  // Sorted by Date ASC.
  const expectedSortedByAltKey = ['1 second', '1 minute', '1 hour'];

  test('sort by key', () => {
    const { container } = render(
      <Table
        data={sample}
        columns={[
          {
            key: 'durationText',
            headerText: 'duration',
            isSortable: true,
          },
        ]}
        emptyText=""
      />
    );

    const cols = container.querySelectorAll('tbody > tr > td');
    expect(cols).toHaveLength(sample.length);

    const vals = [];
    cols.forEach(c => vals.push(c.textContent));
    expect(vals).toStrictEqual(expectedSortedByKey);
  });

  test('sort by key with initialSort', () => {
    const { container } = render(
      <Table
        data={sample}
        columns={[
          // first column
          {
            key: 'hostname',
            headerText: 'hostname',
            isSortable: true,
          },
          // second column
          {
            key: 'durationText',
            headerText: 'duration',
            isSortable: true,
          },
        ]}
        emptyText=""
        initialSort={{ key: 'durationText', dir: 'ASC' }}
      />
    );

    const cols = container.querySelectorAll('tbody > tr > td');
    const vals = [];
    // field durationText starts in the second column,
    // which is every odd number per row.
    cols.forEach((c, i) => i % 2 != 0 && vals.push(c.textContent));
    expect(vals).toHaveLength(sample.length);
    expect(vals).toStrictEqual(expectedSortedByKey);
  });

  test('sort by altSortKey', () => {
    const { container } = render(
      <Table
        data={sample}
        columns={[
          {
            key: 'durationText',
            altSortKey: 'created',
            headerText: 'duration',
            isSortable: true,
          },
        ]}
        emptyText=""
      />
    );

    const cols = container.querySelectorAll('tbody > tr > td');
    expect(cols).toHaveLength(sample.length);

    const vals = [];
    cols.forEach(c => vals.push(c.textContent));
    expect(vals).toStrictEqual(expectedSortedByAltKey);
  });

  test('sort by altSortKey with initialSort', () => {
    const { container } = render(
      <Table
        data={sample}
        columns={[
          // first column
          {
            key: 'hostname',
            headerText: 'hostname',
            isSortable: true,
          },
          // second column
          {
            key: 'durationText',
            altSortKey: 'created',
            headerText: 'duration',
            isSortable: true,
          },
        ]}
        emptyText=""
        initialSort={{ altSortKey: 'created', dir: 'ASC' }}
      />
    );

    const cols = container.querySelectorAll('tbody > tr > td');
    const vals = [];
    // field durationText starts in the second column,
    // which is every odd number per row.
    cols.forEach((c, i) => i % 2 != 0 && vals.push(c.textContent));
    expect(vals).toHaveLength(sample.length);
    expect(vals).toStrictEqual(expectedSortedByAltKey);
  });
});
