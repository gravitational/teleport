import { convertResultToData } from 'e-teleport/AccessMonitoring/Report/BarGraph/utils';
import {
  BarGraphConfig,
  GraphType,
  GroupMode,
} from 'e-teleport/AccessMonitoring/Report/config';
import { ColumnType, ReportResult } from 'e-teleport/AccessMonitoring/types';

describe('convertResultToData', () => {
  const result: ReportResult = {
    name: 'name',
    query: 'query',
    title: 'title',
    description: 'description',
    columns: [
      { name: 'keysColumn', type: ColumnType.VarChar },
      { name: 'indexByColumn', type: ColumnType.VarChar },
      { name: 'valueColumn', type: ColumnType.VarChar },
    ],
    data: [
      ['key1', '2020-01-01T00:00:00.000Z', '1'],
      ['key2', '2020-01-01T00:00:00.000Z', '3'],
      ['key1', '2020-01-02T00:00:00.000Z', '10'],
      ['key2', '2020-01-02T00:00:00.000Z', '15'],
    ],
  };

  const config: BarGraphConfig = {
    groupMode: GroupMode.Stacked,
    name: 'name',
    type: GraphType.Bar,
    keysColumn: 'keysColumn',
    indexByColumn: 'indexByColumn',
    valueColumn: 'valueColumn',
  };

  const from = new Date('2020-01-01');
  const today = new Date('2020-01-02');

  it('should return the correct result', () => {
    const expected = [
      {
        indexByColumn: '2020-01-01T00:00:00.000Z',
        key1: '1',
        key2: '3',
      },
      {
        indexByColumn: '2020-01-02T00:00:00.000Z',
        key1: '10',
        key2: '15',
      },
    ];

    const data = convertResultToData(result, config, null, from, today);

    expect(data.data).toEqual(expected);
  });

  it('should only return the selected key', () => {
    const expected = [
      {
        indexByColumn: '2020-01-01T00:00:00.000Z',
        key1: '1',
      },
      {
        indexByColumn: '2020-01-02T00:00:00.000Z',
        key1: '10',
      },
    ];

    const data = convertResultToData(result, config, 'key1', from, today);

    expect(data.data).toEqual(expected);
  });

  it('should return a map of total counts for each key', () => {
    const expected = new Map<string, number>([
      ['key1', 11],
      ['key2', 18],
    ]);

    const data = convertResultToData(result, config, null, from, today);

    expect(data.counts).toEqual(expected);
  });

  it('should return the total count', () => {
    const expected = 4;

    const data = convertResultToData(result, config, null, from, today);

    expect(data.count).toEqual(expected);
  });
});
