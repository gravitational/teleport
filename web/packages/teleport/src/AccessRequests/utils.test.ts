/*
 * Copyright 2023 Gravitational, Inc.
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

import { Duration } from 'date-fns';

import {
  middleValues,
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
  ];

  test.each(cases)('$name', ({ input, expected }) => {
    const result = roundToNearestTenMinutes(input);
    expect(result).toEqual(expected);
  });
});
