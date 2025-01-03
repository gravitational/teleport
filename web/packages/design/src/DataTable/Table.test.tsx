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

import { within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { fireEvent, render, screen } from 'design/utils/testing';

import { SortIndicator } from './Cells';
import Table from './Table';

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

const getTableRows = () => {
  const [header, ...rows] = screen.getAllByRole('row');
  return { header, rows };
};

describe('design/Table Simple', () => {
  const setup = () =>
    render(
      <Table
        data={data}
        columns={[
          {
            key: 'hostname',
            headerText: 'Hostname',
          },
          {
            key: 'addr',
            headerText: 'Address',
          },
        ]}
        emptyText="No Servers Found"
      />
    );

  test('there is one table element', () => {
    setup();
    expect(screen.getByRole('table')).toBeInTheDocument();
  });

  test('each th tag text == header data', () => {
    setup();

    expect(screen.getByText('Hostname')).toBeInTheDocument();
    expect(screen.getByText('Address')).toBeInTheDocument();
  });

  test('number of tr tags in body == data.length', () => {
    setup();

    const { rows } = getTableRows();

    expect(rows).toHaveLength(data.length);
  });

  test('each td tag text == data texts', () => {
    setup();

    const { rows } = getTableRows();

    rows.forEach((row, index) => {
      expect(within(row).getByText(data[index].addr)).toBeInTheDocument();
      expect(within(row).getByText(data[index].hostname)).toBeInTheDocument();
    });
  });
});

describe('design/Table SortIndicator', () => {
  test('sort indicator defaults to sort vertical (neither ASC or DESC)', () => {
    render(<SortIndicator />);
    expect(screen.getByTitle('sort items')).toHaveClass(
      'icon-chevronsvertical'
    );
  });

  test('sort indicator respects sortDir prop set to ASC', () => {
    render(<SortIndicator sortDir={'ASC'} />);
    expect(screen.getByTitle('sort items asc')).toHaveClass('icon-chevronup');
  });

  test('sort indicator respects sortDir prop set to DESC', () => {
    render(<SortIndicator sortDir={'DESC'} />);
    expect(screen.getByTitle('sort items desc')).toHaveClass(
      'icon-chevrondown'
    );
  });

  test('clicking on col headers changes direction', () => {
    render(
      <Table
        data={data}
        columns={[
          {
            key: 'hostname',
            headerText: 'Hostname',
            isSortable: true,
          },
          {
            key: 'addr',
            headerText: 'Address',
            isSortable: true,
          },
        ]}
        emptyText="No Servers Found"
      />
    );

    // Table initially sorts with "Hostname" ASC
    expect(
      within(screen.getByText('Hostname')).getByTitle('sort items asc')
    ).toBeInTheDocument();

    // b/c Table is initially sorted by "Hostname"
    // "Address" header starts with sort vertical (neither ASC or DESC)
    // on sort vertical, DESC is default
    fireEvent.click(screen.getByText('Hostname'));

    expect(
      within(screen.getByText('Address')).getByTitle('sort items')
    ).toBeInTheDocument();

    fireEvent.click(screen.getByText('Address'));

    expect(
      within(screen.getByText('Address')).getByTitle('sort items asc')
    ).toBeInTheDocument();
  });
});

test('"onSort" prop is respected', () => {
  const dummyFunc = jest.fn(() => -1);

  render(
    <Table
      data={data}
      columns={[
        {
          key: 'hostname',
          headerText: 'Hostname',
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
  render(<Table data={[]} columns={[]} emptyText={targetText} />);
  const target = screen.getByText(targetText);

  expect(target).toHaveTextContent(targetText);
});

describe('sorting by field defined in key and altSortKey', () => {
  const sample = [
    {
      hostname: 'host-a',
      created: new Date('2022-07-15T15:34:33.256697813Z'),
      durationText: '1',
    },
    {
      hostname: 'host-b',
      created: new Date('2022-07-05T15:34:33.256697813Z'),
      durationText: '3',
    },
    {
      hostname: 'host-c',
      created: new Date('2022-07-10T15:34:33.256697813Z'),
      durationText: '2',
    },
  ];

  test('sort by key', () => {
    render(
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

    const cells = screen.getAllByRole('cell');

    expect(cells[0]).toHaveTextContent('1');
    expect(cells[1]).toHaveTextContent('2');
    expect(cells[2]).toHaveTextContent('3');
  });

  test('sort by key with initialSort', () => {
    render(
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
    const { rows } = getTableRows();
    const cells = rows.map(row => within(row).getAllByRole('cell')[1]);

    expect(cells[0]).toHaveTextContent('1');
    expect(cells[1]).toHaveTextContent('2');
    expect(cells[2]).toHaveTextContent('3');
  });

  test('sort by altSortKey', () => {
    render(
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

    const cells = screen.getAllByRole('cell');

    expect(cells[0]).toHaveTextContent('3');
    expect(cells[1]).toHaveTextContent('2');
    expect(cells[2]).toHaveTextContent('1');
  });

  test('sort by altSortKey with initialSort', () => {
    render(
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

    const { rows } = getTableRows();

    const cells = rows.map(row => within(row).getAllByRole('cell')[1]);

    expect(cells[0]).toHaveTextContent('3');
    expect(cells[1]).toHaveTextContent('2');
    expect(cells[2]).toHaveTextContent('1');
  });
});

describe('sorting by iso date string', () => {
  const sample = [
    {
      created: '2022-09-09T19:08:17.27Z',
      text: 'a2',
    },
    {
      created: '2022-09-09T19:08:17.261Z',
      text: 'a3',
    },
    {
      created: new Date('2023-09-09T19:08:17.261Z').toISOString(),
      text: 'a1',
    },
  ];

  test('sort by iso date string', () => {
    render(
      <Table
        data={sample}
        columns={[
          {
            key: 'created',
            headerText: 'created',
            isSortable: true,
          },
          {
            key: 'text',
            headerText: 'text',
            isSortable: false,
          },
        ]}
        emptyText=""
        initialSort={{ key: 'created', dir: 'DESC' }}
      />
    );

    const { rows } = getTableRows();

    expect(rows[0]).toHaveTextContent('a1');
    expect(rows[1]).toHaveTextContent('a2');
    expect(rows[2]).toHaveTextContent('a3');
  });
});

test('navigate to next and previous pages', async () => {
  render(
    <Table
      data={data}
      columns={[
        {
          key: 'hostname',
          headerText: 'created',
        },
      ]}
      emptyText=""
      pagination={{ pageSize: 2 }}
    />
  );

  let { rows } = getTableRows();
  expect(rows).toHaveLength(2);
  expect(rows[0]).toHaveTextContent(data[0].hostname);
  expect(rows[1]).toHaveTextContent(data[1].hostname);

  await userEvent.click(screen.getByTitle('Next page'));
  rows = getTableRows().rows;
  expect(rows).toHaveLength(2);
  expect(rows[0]).toHaveTextContent(data[2].hostname);
  expect(rows[1]).toHaveTextContent(data[3].hostname);

  await userEvent.click(screen.getByTitle('Previous page'));
  rows = getTableRows().rows;
  expect(rows).toHaveLength(2);
  expect(rows[0]).toHaveTextContent(data[0].hostname);
  expect(rows[1]).toHaveTextContent(data[1].hostname);
});
