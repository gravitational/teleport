import { scaleTime } from 'd3-scale';

import { BarGraphConfig } from 'e-teleport/AccessMonitoring/Report/config';
import { ReportResult } from 'e-teleport/AccessMonitoring/types';

export const MAX_RESULTS = 500;
const MAX_TICKS = 20;

// compare two dates without comparing hours/minutes/seconds/milliseconds
function areDatesEqual(date1: Date, date2: Date) {
  return (
    date1.getFullYear() === date2.getFullYear() &&
    date1.getMonth() === date2.getMonth() &&
    date1.getDate() === date2.getDate()
  );
}

export function convertResultToData(
  result: ReportResult,
  config: BarGraphConfig,
  selected: string | null,
  from: Date,
  today: Date
) {
  const keysColumnIndex = result.columns.findIndex(
    column => column.name === config.keysColumn
  );

  const count = result.data.length;

  let rows = result.data;
  if (count > MAX_RESULTS) {
    rows = rows.slice(count - MAX_RESULTS, count);
  }

  const keys = Array.from(new Set(rows.map(row => row[keysColumnIndex])));

  const data: Record<string, string | number>[] = [];

  const indexByColumnIndex = result.columns.findIndex(
    column => column.name === config.indexByColumn
  );

  const valueColumnIndex = result.columns.findIndex(
    column => column.name === config.valueColumn
  );

  // for each day, create a new entry in the data array
  for (let day = new Date(from); day <= today; day.setDate(day.getDate() + 1)) {
    data.push({
      [config.indexByColumn]: day.toISOString(),
    });
  }

  const counts = new Map<string, number>();

  // for each row in the report result, find the corresponding entry in the data array
  // and set the value
  for (const row of rows) {
    let dataObjectIndex = data.findIndex(data => {
      return areDatesEqual(
        new Date(data[config.indexByColumn]),
        new Date(row[indexByColumnIndex])
      );
    });

    if (dataObjectIndex === -1) {
      continue;
    }

    const count = counts.get(row[keysColumnIndex]) || 0;

    counts.set(
      row[keysColumnIndex],
      count + parseInt(row[valueColumnIndex], 10)
    );

    if (selected && row[keysColumnIndex] !== selected) {
      continue;
    }

    let dataObject = data[dataObjectIndex];

    dataObject[row[keysColumnIndex]] = row[valueColumnIndex];
  }

  return {
    count,
    counts,
    keys,
    data,
  };
}

const SECONDS_IN_DAY = 24 * 60 * 60;

export function getTimescale(days: number) {
  // today's date without any hours/minutes/seconds/milliseconds
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  // from is today minus the number of days
  const from = new Date(today.getTime() - (days - 1) * SECONDS_IN_DAY * 1000);

  return { today, from };
}

export function getXAxisTicks(
  from: Date,
  today: Date,
  formatter: (date: Date) => string
) {
  const scale = scaleTime().domain([from, today]);

  return scale.ticks(MAX_TICKS).map(formatter);
}
