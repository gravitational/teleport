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

import {
  addDays,
  addHours,
  Duration,
  intervalToDuration,
  isAfter,
  isBefore,
} from 'date-fns';

type TimeDuration = {
  timestamp: number;
  duration: Duration;
};

// Round the duration to the nearest 10 minutes
// Example:
// 9m -> 10m
// 10m -> 10m
// 11m -> 10m
// 15m -> 20m
// 1d -> 1d
// 1d 1h -> 1d 1h
// The only exception is 0m, which is rounded to 10m
export function roundToNearestTenMinutes(date: Duration): Duration {
  let minutes = date.minutes;
  let roundedMinutes = Math.round(minutes / 10) * 10; // Round to the nearest 10
  if (roundedMinutes === 0 && !date.days && !date.hours) {
    // Do not round down to 0. This
    roundedMinutes = 10;
  }

  // 60 minutes == 1 hour
  // Prevent displaying time as eg: `5 hrs and 60 mins`
  // to `6 hrs`.
  if (roundedMinutes === 60) {
    if (!date.hours) {
      date.hours = 0;
    }
    date.hours += 1;
    roundedMinutes = 0;
  }
  date.minutes = roundedMinutes;
  date.seconds = 0;

  return date;
}

// Generate a list of middle values between start and end. The first value is the
// session TTL that is rounded to the nearest hour. The rest of the values are
// rounded to the nearest day. Example:
//
// created: 2021-09-01T00:00:00.000Z
// start: 2021-09-01T01:00:00.000Z
// end: 2021-09-03T00:00:00.000Z
// now: 2021-09-01T00:00:00.000Z
//
// returns: [1h, 1d, 2d, 3d]
export function middleValues(
  created: Date,
  start: Date,
  end: Date
): TimeDuration[] {
  const getInterval = (d: Date) =>
    roundToNearestTenMinutes(
      intervalToDuration({
        start: created,
        end: d,
      })
    );

  const points: Date[] = [start];

  if (isAfter(addDays(created, 1), end)) {
    // Add all possible options to the list. This covers the case when the
    // max duration is less than 24 hours.
    if (isBefore(addHours(points[points.length - 1], 1), end)) {
      points.push(end);
    }

    return points.map(d => ({
      timestamp: d.getTime(),
      duration: getInterval(d),
    }));
  }

  points.push(addDays(created, 1));

  // I also prefer while(true), but our linter doesn't
  for (;;) {
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

// Generate a list of middle values between now and the request TTL.
export function requestTtlMiddleValues(
  created: Date,
  requestTtl: Date
): TimeDuration[] {
  const getInterval = (d: Date) =>
    roundToNearestTenMinutes(
      intervalToDuration({
        start: created,
        end: d,
      })
    );

  if (isAfter(addHours(created, 1), requestTtl)) {
    return [
      {
        timestamp: requestTtl.getTime(),
        duration: getInterval(requestTtl),
      },
    ];
  }

  const points: Date[] = [];
  // Staggered hour options, up to 1 week.
  const hourOptions = [1, 2, 3, 4, 6, 8, 12, 18, 24, 30, 168];

  for (const h of hourOptions) {
    const t = addHours(created, h);
    if (isAfter(t, requestTtl)) {
      break;
    }
    points.push(t);
  }

  return points.map(d => ({
    timestamp: d.getTime(),
    duration: getInterval(d),
  }));
}
