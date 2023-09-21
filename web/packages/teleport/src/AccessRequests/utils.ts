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

import {
  addDays,
  addHours,
  Duration,
  intervalToDuration,
  isAfter,
} from 'date-fns';

type TimeDuration = {
  timestamp: number;
  duration: Duration;
};

// Generate a list of middle values between start and end. The first value is the
// session TTL that is rounded to the nearest hour. The rest of the values are
// rounded to the nearest day. Example:
//
// start: 2021-09-01T01:00:00.000Z
// end: 2021-09-03T00:00:00.000Z
// now: 2021-09-01T00:00:00.000Z
//
// returns: [1h, 1d, 2d, 3d]
export function middleValues(created: Date, start: Date, end: Date): TimeDuration[] {
  const getInterval = (d: Date) =>
    intervalToDuration({
      start: created,
      end: d,
    });

  const points: Date[] = [start];

  if (isAfter(addDays(created, 1), end)) {
    return points.map(d => ({
      timestamp: d.getTime(),
      duration: getInterval(d),
    }));
  }

  points.push(addDays(created, 1));

  while (true) {
    const next = addHours(points[points.length - 1], 24);
    // Allow next == end
    if (next > end) {
      break;
    }
    points.push(next);
  }

  return points.map(d => ({
    timestamp: d.getTime(),
    duration: getInterval(d),
  }));
}
