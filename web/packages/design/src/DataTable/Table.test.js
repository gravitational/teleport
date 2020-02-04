import React from 'react';
import {
  Table,
  TextCell,
  Cell,
  Column,
  SortIndicator,
  SortTypes,
  SortHeaderCell,
  EmptyIndicator,
} from './index';
import { TableSample, data as TableSampleData } from './Table.story';
import { render, fireEvent, screen } from 'design/utils/testing';

const colHeaderKeys = ['hostname', 'addr'];
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
      <Table data={data}>
        <Column
          columnKey={colHeaderKeys[0]}
          header={<Cell>{colHeaders[0]}</Cell>}
          cell={<TextCell />}
        />
        <Column
          columnKey={colHeaderKeys[1]}
          header={<Cell>{colHeaders[1]}</Cell>}
          cell={<TextCell />}
        />
      </Table>
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
    const { container } = render(<SortIndicator sortDir={SortTypes.ASC} />);
    expect(
      container.querySelector('span').classList.contains('icon-chevron-up')
    ).toBe(true);
  });

  test('sort indicator respects sortDir prop set to DESC', () => {
    const { container } = render(<SortIndicator sortDir={SortTypes.DESC} />);
    expect(
      container.querySelector('span').classList.contains('icon-chevron-down')
    ).toBe(true);
  });

  test('clicking on col headers changes direction', () => {
    const tableSampleHeaders = ['Hostname', 'Address'];

    const { container } = render(
      <TableSample TableComponent={Table} data={TableSampleData} />
    );

    const anchorTags = container.querySelectorAll('a');
    const header1 = anchorTags[0];
    const header2 = anchorTags[1];

    // TableSample initially sorts with "Hostname" DESC
    fireEvent.click(screen.getByText(tableSampleHeaders[0]));
    expect(
      header1.querySelector('span').classList.contains('icon-chevron-up')
    ).toBe(true);

    // b/c TableSample is initially sorted by "Hostname"
    // "Address" header starts with sort vertical (neither ASC or DESC)
    // on sort vertical, DESC is default
    fireEvent.click(screen.getByText(tableSampleHeaders[1]));
    expect(
      header2.querySelector('span').classList.contains('icon-chevron-down')
    ).toBe(true);

    fireEvent.click(screen.getByText(tableSampleHeaders[1]));
    expect(
      header2.querySelector('span').classList.contains('icon-chevron-up')
    ).toBe(true);
  });
});

describe('design/Table SortHeaderCell', () => {
  test('"onSortChange" prop is respected', () => {
    const dummyFunc = jest.fn((columnKey, newDir) => [columnKey, newDir]);

    const tableTitle = 'some title';
    const colKey = 'col key';
    const retVal = [colKey, SortTypes.DESC];

    render(
      <Table data={[]}>
        <Column
          columnKey={colKey}
          header={
            <SortHeaderCell title={tableTitle} onSortChange={dummyFunc} />
          }
          cell={<TextCell />}
        />
      </Table>
    );

    fireEvent.click(screen.getByText(tableTitle));
    expect(dummyFunc).toHaveReturnedWith(retVal);
  });

  test('"onSortChange" empty prop does nothing', () => {
    const dummyFunc = jest.fn(() => true);

    const tableTitle = 'some title';
    render(
      <Table data={[]}>
        <Column
          header={<SortHeaderCell title={tableTitle} />}
          cell={<TextCell />}
        />
      </Table>
    );

    fireEvent.click(screen.getByText(tableTitle));
    expect(dummyFunc).not.toHaveReturned();
  });
});

describe('design/Table Empty && EmptyIndicator', () => {
  test('empty table data is handled', () => {
    const targetText = 'NO DATA AVAILABLE';
    const { getByText } = render(<Table data={[]} />);
    const target = getByText(targetText);

    expect(target.textContent).toEqual(targetText);
  });

  test('no prop EmptyIndicator', () => {
    const targetText = 'No Results Found';
    const { getByText } = render(<EmptyIndicator />);
    const target = getByText(targetText);

    expect(target.textContent).toEqual(targetText);
  });

  it('respects title prop and children', () => {
    const title = 'Testing abc';
    const children = 'testing children';
    const { getByText } = render(
      <EmptyIndicator title={title}>{children}</EmptyIndicator>
    );
    const targetTitle = getByText(title);
    const targetChildren = getByText(children);

    expect(targetTitle.textContent).toEqual(title);
    expect(targetChildren.textContent).toEqual(children);
  });
});
