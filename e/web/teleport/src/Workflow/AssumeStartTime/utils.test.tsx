import { addHours, addWeeks, addDays } from 'date-fns';

import {
  getStandardHoursAndPostfix,
  getDurationOptionIndexClosestToOneWeek,
  DurationOption,
  getMaxAssumableDate,
  OneWeek,
} from './utils';

describe('getStandardHoursAndPostfix', () => {
  test.each`
    militaryHrs | expectedStandardHrs | expectedPostfix
    ${0}        | ${12}               | ${'AM'}
    ${1}        | ${1}                | ${'AM'}
    ${12}       | ${12}               | ${'PM'}
    ${13}       | ${1}                | ${'PM'}
    ${23}       | ${11}               | ${'PM'}
  `(
    'militaryHrs "$militaryHrs" should "$expectedStandardHrs $expectedPostfix"',
    ({ militaryHrs, expectedStandardHrs, expectedPostfix }) => {
      const h = getStandardHoursAndPostfix(militaryHrs);
      expect(h).toEqual({ ampm: expectedPostfix, hours: expectedStandardHrs });
    }
  );
});

describe('getDurationOptionIndexClosestToOneWeek', () => {
  const beginDate = new Date('2024-02-10T03:00:00.000000Z');
  jest.useFakeTimers().setSystemTime(beginDate);

  const durationOpts: DurationOption[] = [
    { value: beginDate.getTime(), label: '' }, // earliest date
    { value: addHours(beginDate, 3).getTime(), label: '' },
    { value: addHours(beginDate, 6).getTime(), label: '' },
    { value: addHours(beginDate, 9).getTime(), label: '' },
    { value: addDays(beginDate, 3).getTime(), label: '' },
    { value: addDays(beginDate, 7).getTime(), label: '' }, //  one week
    { value: addDays(beginDate, 8).getTime(), label: '' },
    { value: addDays(beginDate, 10).getTime(), label: '' },
    { value: addWeeks(beginDate, 2).getTime(), label: '' }, // two week
  ];

  const lastDurationIndex = durationOpts.length - 1;

  test('one week from selected date, is greater than value from last index, returns the last index', () => {
    const startDate = addDays(beginDate, 10);

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate // 1 week from startDate is 17 days, past 2 weeks.
    );
    expect(index).toBe(lastDurationIndex);
  });

  test('one week from selected date, is equal to the value from last index, returns the last index', () => {
    const startDate = addWeeks(beginDate, 1);

    // Ensure the expected option is what we expect.
    expect(durationOpts[lastDurationIndex].value).toBe(
      addWeeks(startDate, 1).getTime()
    );

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate // 1 week from start date is exactly 2 weeks
    );
    expect(index).toBe(lastDurationIndex);
  });

  test('one week from selected date, is less than the last index, returns the index equal to one week', () => {
    const startDate = beginDate;
    const expectedIndex = 5;

    // Ensure the expected option is what we expect.
    expect(durationOpts[expectedIndex].value).toBe(
      addWeeks(beginDate, 1).getTime() // 1 week from start date is exactly 1 week
    );

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate
    );
    expect(index).toBe(expectedIndex);
  });

  test('one week from selected date, is less than the last index, returns the index closest but no greater than one week', () => {
    const startDate = addDays(beginDate, 3);
    const expectedIndex = 7;

    // Ensure the expected option is what we expect.
    expect(durationOpts[expectedIndex].value).toBe(
      addDays(beginDate, 10).getTime() // 1 week from start date is day 10
    );

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate
    );
    expect(index).toBe(expectedIndex);
  });
});

test('getMaxAssumableDate', () => {
  const created = new Date('2024-02-01T03:00:00.000000Z');
  jest.useFakeTimers().setSystemTime(created);

  // max date is greater than 1 week added to begin date.
  let maxDuration = new Date('2024-02-20T03:00:00.000000Z');
  expect(getMaxAssumableDate({ created, maxDuration })).toEqual(
    addDays(created, OneWeek)
  );

  // max date is lesser than 1 week added to begin date.
  maxDuration = new Date('2024-02-03T03:00:00.000000Z');
  expect(getMaxAssumableDate({ created, maxDuration })).toEqual(maxDuration);
});
