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

import { middleValues } from 'e-teleport/Workflow/NewRequest/RequestCheckout/timeHelpers';

beforeAll(() => {
  jest.useFakeTimers('modern');
  jest.setSystemTime(new Date(2021, 8, 1));
});

afterAll(() => {
  jest.useRealTimers();
});

test('generate middle times', () => {
  const dataFiller = {
    years: 0,
    months: 0,
    minutes: 0,
    seconds: 0,
  };
  const result = middleValues(
    new Date('2021-09-01T01:00:00.000Z'),
    new Date('2021-09-03T00:00:00.000Z')
  );
  expect(result).toEqual([
    {
      timestamp: 1630458000000,
      duration: {
        days: 0,
        hours: 1,
        ...dataFiller,
      },
    },
    {
      timestamp: 1630540800000,
      duration: {
        days: 1,
        hours: 0,
        ...dataFiller,
      },
    },
    {
      timestamp: 1630627200000,
      duration: {
        days: 2,
        hours: 0,
        ...dataFiller,
      },
    },
    {
      timestamp: 1630713600000,
      duration: {
        days: 3,
        hours: 0,
        ...dataFiller,
      },
    },
  ]);
});
