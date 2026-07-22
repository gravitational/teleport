/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
  addWeeks,
  differenceInHours,
  isAfter,
} from 'date-fns';

import { Option } from 'shared/components/Select';
import { AccessRequest } from 'shared/services/accessRequests';

import { getFormattedDurationTxt } from '../Shared/utils';

// Preset hour options for the access duration dropdown.
export const PRESET_HOURS = [1, 2, 3, 4, 6, 8, 12, 18];
// Preset day options, up to the maximum possible duration, for
// the access duration dropdown. The backend maximum duration
// is two weeks.
export const PRESET_DAYS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14];

export type DurationOption = Option<number>;

function getMaxDurationOption(
  startDateTime: Date,
  maxDuration: Date
): Option<number> {
  return {
    value: maxDuration.getTime(),
    label: getFormattedDurationTxt({
      start: startDateTime,
      end: maxDuration,
    }),
  };
}

/**
 * Calculates the selectable access duration options depending
 * on the selected start time. The duration range is starting from the
 * "selected start time" to the request's "max duration".
 *
 * Access duration refers to how long access should last after
 * start date/time.
 */
export function getDurationOptionsFromStartTime(
  startDate: Date | null,
  accessRequest: AccessRequest
): DurationOption[] {
  const start = startDate || accessRequest.created;
  // The setSeconds(0,0) removes the seconds and milliseconds since
  // `startDateTime` is constructed without them. Makes comparing difference
  // in hours ignore the small time difference.
  const createdTimestamp = new Date(accessRequest.created).setSeconds(0, 0);
  const maxDurationTimestamp = new Date(accessRequest.maxDuration).setSeconds(
    0,
    0
  );

  if (
    start.getTime() >= maxDurationTimestamp ||
    start.getTime() < createdTimestamp
  ) {
    return [];
  }

  let durationOpts: Option<number>[] = [];

  const totalHoursDiff = differenceInHours(
    maxDurationTimestamp,
    start.getTime(),
    {
      roundingMethod: 'ceil',
    }
  );

  // If there is less than an hour available for access, return
  // it as max duration as the only option.
  if (totalHoursDiff <= 1) {
    durationOpts.push(getMaxDurationOption(start, accessRequest.maxDuration));
    return durationOpts;
  }

  // Add preset hour options up to maximum allowed.
  for (const hour of PRESET_HOURS) {
    const updatedDateTime = addHours(start, hour);
    if (isAfter(updatedDateTime, accessRequest.maxDuration)) {
      break;
    }
    durationOpts.push({
      value: updatedDateTime.getTime(),
      label: getFormattedDurationTxt({
        start,
        end: updatedDateTime,
      }),
    });
  }

  // Add preset days up to maximum allowed.
  if (totalHoursDiff >= 24) {
    for (const day of PRESET_DAYS) {
      const updatedEndDate = addDays(start, day);
      if (isAfter(updatedEndDate, accessRequest.maxDuration)) {
        break;
      }
      durationOpts.push({
        value: updatedEndDate.getTime(),
        label: getFormattedDurationTxt({
          start,
          end: updatedEndDate,
        }),
      });
    }
  }

  const lastDurationOption = durationOpts[durationOpts.length - 1];
  if (maxDurationTimestamp > lastDurationOption.value) {
    durationOpts.push(getMaxDurationOption(start, accessRequest.maxDuration));
  }

  return durationOpts;
}

// Goes through the given duration options and returns the index
// that is closest to one week from given start date.
// It was decided that one week is a good default duration
// to pre-select for the user for the following duration types:
//  - Access duration ranges can go as high as 14 days.
//  - Pending request duration (how long the request should be
//    in the pending state before it expires) can go as high as
//    7 days. The use of this function for this case just guards
//    against future increase.
export function getDurationOptionIndexClosestToOneWeek(
  durationOptions: DurationOption[],
  startDate: Date
) {
  const oneWeekFromSelectedTime = addWeeks(startDate, 1).getTime();
  const lastDurationIndex = durationOptions.length - 1;

  // Default to the last option, since that is the max the user can get.
  if (oneWeekFromSelectedTime >= durationOptions[lastDurationIndex].value) {
    return lastDurationIndex;
  }

  // Find an option that is nearest to one week, but no greater.
  let closestIndex = 0;
  for (let i = 0; i < durationOptions.length; i++) {
    const currentTime = durationOptions[i].value;
    if (currentTime === oneWeekFromSelectedTime) {
      closestIndex = i;
      break;
    } else if (currentTime > oneWeekFromSelectedTime) {
      // the last stored index was closest to the one week but no greater
      break;
    }
    closestIndex = i;
  }

  return closestIndex;
}
