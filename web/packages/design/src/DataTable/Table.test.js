import React from 'react';
import { Table, TextCell, Cell, Column } from './index';
import { render } from 'design/utils/testing';

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

describe('Design/Table', () => {
  let container = null;
  let debug = null;

  beforeEach(() => {
    ({ container, debug } = render(
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
