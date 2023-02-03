import paginateData from './paginateData';

test('paginates data correctly given pageSize', () => {
  const pageSize = 4;
  const paginatedData = paginateData(data, pageSize);

  expect(paginatedData).toEqual([
    [
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
    ],
    [
      {
        hostname: 'host-4',
        addr: '192.168.7.4',
      },
      {
        hostname: 'host-e',
        addr: '192.168.7.1',
      },
      {
        hostname: 'host-f',
        addr: '192.168.7.2',
      },
      {
        hostname: 'host-g',
        addr: '192.168.7.3',
      },
    ],
    [
      {
        hostname: 'host-h',
        addr: '192.168.7.4',
      },
      {
        hostname: 'host-i',
        addr: '192.168.7.4',
      },
    ],
  ]);
});

test('empty data set should return array with an empty array inside it', () => {
  const paginatedData = paginateData([], 5);

  expect(paginatedData).toEqual([[]]);
});

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
    hostname: 'host-4',
    addr: '192.168.7.4',
  },
  {
    hostname: 'host-e',
    addr: '192.168.7.1',
  },
  {
    hostname: 'host-f',
    addr: '192.168.7.2',
  },
  {
    hostname: 'host-g',
    addr: '192.168.7.3',
  },
  {
    hostname: 'host-h',
    addr: '192.168.7.4',
  },
  {
    hostname: 'host-i',
    addr: '192.168.7.4',
  },
];
