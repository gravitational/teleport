import { addDays } from 'date-fns';

import { getMaxAssumableDate, OneWeek } from './timeOptions';

test('getMaxAssumableDate, return the lesser date between max duration and default with 1 hour subtracted', () => {
  const created = new Date('2024-02-01T03:00:00.000000Z');
  const defaultMaxAssumableDate = addDays(created, OneWeek);
  jest.useFakeTimers().setSystemTime(created);

  // Return the default when max duration is greater.
  let maxDuration = new Date('2024-02-20T03:00:00.000000Z');
  let gotDate = getMaxAssumableDate({ created, maxDuration });
  expect(gotDate.getTime()).toEqual(
    defaultMaxAssumableDate.setHours(defaultMaxAssumableDate.getHours() - 1)
  );

  // Return max duration when default is greater.
  maxDuration = new Date('2024-02-03T03:00:00.000000Z');
  gotDate = getMaxAssumableDate({ created, maxDuration });
  expect(gotDate.getTime()).toEqual(
    maxDuration.setHours(maxDuration.getHours() - 1)
  );
});

test('getMaxAssumableDate, returns unmodified date if max duration is less than an hour', () => {
  const created = new Date('2024-02-01T03:00:00.000000Z');
  const maxDuration = new Date('2024-02-01T03:30:00.000000Z'); // 30 min diff
  jest.useFakeTimers().setSystemTime(created);

  const gotDate = getMaxAssumableDate({ created, maxDuration });
  expect(gotDate).toEqual(maxDuration);
});
