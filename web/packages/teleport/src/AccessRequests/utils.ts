/**
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

interface TimeDuration {
  timestamp: number;
  duration: Duration;
}

export function middleValues(start: Date, end: Date): TimeDuration[] {
  const now = new Date();

  const roundDuration = (d: Date) =>
    roundToNearestHour(
      intervalToDuration({
        start: now,
        end: d,
      })
    );

  const points: Date[] = [start];

  if (isAfter(addDays(start, 1), end)) {
    return points.map(d => ({
      timestamp: d.getTime(),
      duration: roundDuration(d),
    }));
  }

  points.push(addDays(now, 1));

  while (points[points.length - 1] <= end) {
    points.push(addHours(points[points.length - 1], 24));
  }

  return points.map(d => ({
    timestamp: d.getTime(),
    duration: roundDuration(d),
  }));
}

export function roundToNearestHour(duration: Duration): Duration {
  if (duration.minutes > 30) {
    duration.hours += 1;
  }

  if (duration.hours >= 24) {
    duration.days += 1;
    duration.hours -= 24;
  }

  duration.minutes = 0;
  duration.seconds = 0;

  return duration;
}
