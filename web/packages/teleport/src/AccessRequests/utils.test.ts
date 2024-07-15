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

import { Duration } from 'date-fns';

import {
  middleValues,
  requestTtlMiddleValues,
  roundToNearestTenMinutes,
} from 'teleport/AccessRequests/utils';

// Generate testing response
function generateResponse(
  currentDate: Date,
  values: Array<{
    days: number;
    hours: number;
    minutes: number;
  }>
) {
  const defaultValues = {
    years: 0,
    months: 0,
    minutes: 0,
    seconds: 0,
  };
  const result = [];
  for (let i = 0; i < values.length; i++) {
    const { days, hours, minutes } = values[i];
    const duration = {
      ...defaultValues,
      days,
      hours,
      minutes,
    };
    let d = new Date(currentDate);
    d.setDate(currentDate.getDate() + days);
    d.setHours(currentDate.getHours() + hours);
    d.setMinutes(currentDate.getMinutes() + minutes);
    const timestamp = d.getTime();
    result.push({
      timestamp,
      duration,
    });
  }
  return result;
}

describe('generate middle times', () => {
  const cases: {
    name: string;
    created: string;
    sessionTTL: string;
    maxDuration: string;
    expected: Array<{
      days: number;
      hours: number;
      minutes: number;
    }>;
  }[] = [
    {
      name: '3 days max',
      created: '2021-09-01T00:00:00.000Z',
      sessionTTL: '2021-09-01T01:00:00.000Z',
      maxDuration: '2021-09-04T00:00:00.000Z',
      expected: [
        {
          days: 0,
          hours: 1,
          minutes: 0,
        },
        {
          days: 1,
          hours: 0,
          minutes: 0,
        },
        {
          days: 2,
          hours: 0,
          minutes: 0,
        },
        {
          days: 3,
          hours: 0,
          minutes: 0,
        },
      ],
    },
    {
      name: '1 day max',
      created: '2021-09-01T00:00:00.000Z',
      sessionTTL: '2021-09-01T01:00:00.000Z',
      maxDuration: '2021-09-02T00:00:00.000Z',
      expected: [
        {
          days: 0,
          hours: 1,
          minutes: 0,
        },
        {
          days: 1,
          hours: 0,
          minutes: 0,
        },
      ],
    },
    {
      name: 'session ttl is 10 min',
      created: '2021-09-01T00:00:00.000Z',
      sessionTTL: '2021-09-01T00:10:00.000Z',
      maxDuration: '2021-09-03T00:00:00.000Z',
      expected: [
        {
          days: 0,
          hours: 0,
          minutes: 10,
        },
        {
          days: 1,
          hours: 0,
          minutes: 0,
        },
        {
          days: 2,
          hours: 0,
          minutes: 0,
        },
      ],
    },
    {
      name: '10 minutes min - real values',
      created: '2023-09-21T20:50:52.669012121Z',
      sessionTTL: '2023-09-21T21:00:52.669081473Z',
      maxDuration: '2023-09-27T20:50:52.669081473Z',
      expected: [
        {
          days: 0,
          hours: 0,
          minutes: 10,
        },
        {
          days: 1,
          hours: 0,
          minutes: 0,
        },
        {
          days: 2,
          hours: 0,
          minutes: 0,
        },
        {
          days: 3,
          hours: 0,
          minutes: 0,
        },
        {
          days: 4,
          hours: 0,
          minutes: 0,
        },
        {
          days: 5,
          hours: 0,
          minutes: 0,
        },
        {
          days: 6,
          hours: 0,
          minutes: 0,
        },
      ],
    },
    {
      name: 'only one option generated',
      created: '2023-09-21T10:00:52.669012121Z',
      sessionTTL: '2023-09-21T15:00:52.669081473Z',
      maxDuration: '2023-09-21T15:00:52.669081473Z',
      expected: [
        {
          days: 0,
          hours: 5,
          minutes: 0,
        },
      ],
    },
    {
      name: 'generate all options if max duration is grater than session ttl but less than 1d',
      created: '2023-09-21T10:00:52.669012121Z',
      sessionTTL: '2023-09-21T15:00:52.669081473Z',
      maxDuration: '2023-09-21T17:00:52.669081473Z',
      expected: [
        {
          days: 0,
          hours: 5,
          minutes: 0,
        },
        {
          days: 0,
          hours: 7,
          minutes: 0,
        },
      ],
    },
  ];

  test.each(cases)(
    '$name',
    ({ sessionTTL, maxDuration, created, expected }) => {
      const result = middleValues(
        new Date(created),
        new Date(sessionTTL),
        new Date(maxDuration)
      );
      expect(result).toEqual(generateResponse(new Date(created), expected));
    }
  );
});

describe('generate request TTL middle times', () => {
  const cases: {
    name: string;
    created: string;
    sessionTTL: string;
    expected: Array<{
      days: number;
      hours: number;
      minutes: number;
    }>;
  }[] = [
    {
      name: 'max session TTL',
      created: '2021-09-01T00:00:00.000Z',
      sessionTTL: '2021-09-02T06:00:00.000Z',
      expected: [
        {
          days: 0,
          hours: 1,
          minutes: 0,
        },
        {
          days: 0,
          hours: 2,
          minutes: 0,
        },
        {
          days: 0,
          hours: 3,
          minutes: 0,
        },
        {
          days: 0,
          hours: 4,
          minutes: 0,
        },
        {
          days: 0,
          hours: 6,
          minutes: 0,
        },
        {
          days: 0,
          hours: 8,
          minutes: 0,
        },
        {
          days: 0,
          hours: 12,
          minutes: 0,
        },
        {
          days: 0,
          hours: 18,
          minutes: 0,
        },
        {
          days: 1,
          hours: 0,
          minutes: 0,
        },
        {
          days: 1,
          hours: 6,
          minutes: 0,
        },
      ],
    },
    {
      name: 'shortest session TTL',
      created: '2021-09-01T00:00:00.000Z',
      sessionTTL: '2021-09-01T00:30:00.000Z',
      expected: [
        {
          days: 0,
          hours: 0,
          minutes: 30,
        },
      ],
    },
    {
      name: 'session TTL in middle',
      created: '2021-09-01T00:00:00.000Z',
      sessionTTL: '2021-09-01T08:30:00.000Z',
      expected: [
        {
          days: 0,
          hours: 1,
          minutes: 0,
        },
        {
          days: 0,
          hours: 2,
          minutes: 0,
        },
        {
          days: 0,
          hours: 3,
          minutes: 0,
        },
        {
          days: 0,
          hours: 4,
          minutes: 0,
        },
        {
          days: 0,
          hours: 6,
          minutes: 0,
        },
        {
          days: 0,
          hours: 8,
          minutes: 0,
        },
      ],
    },
  ];
  test.each(cases)('$name', ({ sessionTTL, created, expected }) => {
    const result = requestTtlMiddleValues(
      new Date(created),
      new Date(sessionTTL)
    );
    expect(result).toEqual(generateResponse(new Date(created), expected));
  });
});

describe('round to nearest 10 minutes', () => {
  const cases: {
    name: string;
    input: Duration;
    expected: Duration;
  }[] = [
    {
      name: 'round up',
      input: { minutes: 9, seconds: 0 },
      expected: { minutes: 10, seconds: 0 },
    },
    {
      name: 'round down',
      input: { minutes: 11, seconds: 0 },
      expected: { minutes: 10, seconds: 0 },
    },
    {
      name: 'round to 10',
      input: { minutes: 15, seconds: 0 },
      expected: { minutes: 20, seconds: 0 },
    },
    {
      name: 'do not round to 0',
      input: { minutes: 1, seconds: 0 },
      expected: { minutes: 10, seconds: 0 },
    },
    {
      name: 'round minutes to 0 when days or hours are present',
      input: { hours: 3, minutes: 1, seconds: 0 },
      expected: { hours: 3, minutes: 0, seconds: 0 },
    },
    {
      name: 'do not round to 0',
      input: { minutes: 0, seconds: 0 },
      expected: { minutes: 10, seconds: 0 },
    },
    {
      name: 'seconds are removed',
      input: { minutes: 9, seconds: 10 },
      expected: { minutes: 10, seconds: 0 },
    },
    {
      name: "duration doesn't change when days are present",
      input: { days: 1, minutes: 9, seconds: 10 },
      expected: { days: 1, minutes: 10, seconds: 0 },
    },
    {
      name: 'add 60 minutes to undefined hour and set minutes to 0',
      input: { minutes: 60 },
      expected: { hours: 1, minutes: 0, seconds: 0 },
    },
    {
      name: 'add 60 minutes to existing hour and set minutes to 0',
      input: { hours: 3, minutes: 60 },
      expected: { hours: 4, minutes: 0, seconds: 0 },
    },
  ];

  test.each(cases)('$name', ({ input, expected }) => {
    const result = roundToNearestTenMinutes(input);
    expect(result).toEqual(expected);
  });
});
