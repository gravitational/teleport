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

import { middleValues } from 'teleport/AccessRequests/utils';

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
  ];

  for (let tc of cases) {
    const { name, sessionTTL, maxDuration, created, expected } = tc;
    test(name, () => {
      const result = middleValues(
        new Date(created),
        new Date(sessionTTL),
        new Date(maxDuration)
      );
      expect(result).toEqual(generateResponse(new Date(created), expected));
    });
  }
});
