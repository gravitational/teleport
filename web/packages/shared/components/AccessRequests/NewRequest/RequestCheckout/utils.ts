/**
 * Copyright 2024 Gravitational, Inc.
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

import { addHours, addDays, isAfter } from 'date-fns';

import { Option } from 'shared/components/Select';

import { getFormattedDurationTxt } from '../../Shared/utils';

// Preset hour options for the time option dropdowns.
export const presetHours = [1, 2, 3, 4, 6, 8, 12, 18];
// Preset day options for the time option dropdowns.
export const presetDays = [1, 2, 3, 4, 5, 6, 7];
const HourInMs = 60 * 60 * 1000;

export function getPendingRequestDurationOptions(
  accessRequestCreated: Date,
  maxDuration: number
): Option<number>[] {
  const createdDate = new Date(accessRequestCreated);

  // Backend limits max pending date to be 1 week.
  // MaxDuration can get greater than 1 week.
  // Pick the smaller of the two.
  let maxPendingDate = new Date(maxDuration);
  const maxPendingDay = presetDays.length - 1;
  const possiblySmallerMaxPendingDate = addDays(
    createdDate,
    presetDays[maxPendingDay]
  );
  if (maxPendingDate.getTime() > possiblySmallerMaxPendingDate.getTime()) {
    maxPendingDate = possiblySmallerMaxPendingDate;
  }

  const createdTimestamp = createdDate.getTime();
  const pendingTimestamp = maxPendingDate.getTime();

  let durationOpts: Option<number>[] = [];

  const totalHoursDiff = (pendingTimestamp - createdTimestamp) / HourInMs;

  // If there is less than an hour available, return
  // it as max duration as the only option.
  if (totalHoursDiff <= 1) {
    durationOpts.push({
      value: pendingTimestamp,
      label: getFormattedDurationTxt({
        start: createdDate,
        end: maxPendingDate,
      }),
    });
    return durationOpts;
  }

  // Add preset hour options up to maximum allowed.
  for (const hour of presetHours) {
    const updatedDateTime = addHours(createdDate, hour);
    if (isAfter(updatedDateTime, maxPendingDate)) {
      break;
    }
    durationOpts.push({
      value: updatedDateTime.getTime(),
      label: getFormattedDurationTxt({
        start: createdDate,
        end: updatedDateTime,
      }),
    });
  }

  // Add preset days up to maximum allowed.
  if (totalHoursDiff >= 24) {
    for (const day of presetDays) {
      const updatedEndDate = addDays(createdDate, day);
      if (isAfter(updatedEndDate, maxPendingDate)) {
        break;
      }
      durationOpts.push({
        value: updatedEndDate.getTime(),
        label: getFormattedDurationTxt({
          start: createdDate,
          end: updatedEndDate,
        }),
      });
    }
  }

  const lastDurationOption = durationOpts[durationOpts.length - 1];
  if (pendingTimestamp > lastDurationOption.value) {
    durationOpts.push({
      value: pendingTimestamp,
      label: getFormattedDurationTxt({
        start: createdDate,
        end: maxPendingDate,
      }),
    });
  }

  return durationOpts;
}

/**
 * Backend expects the maxDuration field to be set to some value
 * which on dry run's will get overwritten to whatever the backend
 * defualt max maxDuration is. Leaving maxDuration field undefined
 * for dry run's, for some reason gets respected.
 */
export function getDryRunMaxDuration() {
  const sevenDaysInMs = 1000 * 60 * 60 * 24 * 7;
  return new Date(Date.now() + sevenDaysInMs);
}
