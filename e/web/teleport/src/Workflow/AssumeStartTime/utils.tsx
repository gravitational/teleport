import { addHours, addDays, isAfter, addWeeks } from 'date-fns';
import { Option } from 'shared/components/Select';

import { AccessRequest } from 'e-teleport/services/workflow/types';

import { getFormattedDurationTxt, getStartDateTime } from '../Shared/utils';
import { Start, TimeOption } from '../Shared/types';

const OneDayInMinutes = 1440; // 24 hours
const HourInMs = 60 * 60 * 1000;
export const OneWeek = 7;

// DateTimeLimit defines the earliest a time can start
// and the latest time can end.
type DateTimeLimit = {
  minTimestamp?: number;
  maxTimestamp?: number;
  startDate: Date;
};

/**
 * isWithinTimeLimit checks if the current timestamp
 * is within the limits of min and max timestamps.
 * Returns false if current timestamp is out of min/max range.
 */
function isWithinTimeLimit(limit: DateTimeLimit, currentTimestamp: number) {
  if (limit.minTimestamp && limit.maxTimestamp) {
    return (
      currentTimestamp >= limit.minTimestamp &&
      currentTimestamp <= limit.maxTimestamp
    );
  }

  if (limit.minTimestamp) {
    return currentTimestamp >= limit.minTimestamp;
  }

  if (limit.maxTimestamp) {
    return currentTimestamp <= limit.maxTimestamp;
  }

  return true;
}

/**
 * generateTimeDropdown generates time options in format 00:00 AM|PM
 * Time can start as early as 12:00 AM and can be as late as 11:30 PM
 * (if `incrementBy = 30` for example)
 *
 * The range of options is defined by the limit param.
 */
export function generateTimeDropdown(
  limit: DateTimeLimit,
  incrementTimeBy = 60 // default to incrementing time by the hour
) {
  const times: TimeOption[] = [];

  for (let i = 0; i < OneDayInMinutes; i += incrementTimeBy) {
    const militaryHrs = Math.floor(i / 60);
    const minutes = i % 60;

    const currentDate = new Date(limit.startDate);
    if (!isWithinTimeLimit(limit, currentDate.setHours(militaryHrs, minutes))) {
      continue;
    }

    const { hours, ampm } = getStandardHoursAndPostfix(Math.floor(i / 60));
    times.push({
      label: `${hours}:${String(minutes).padStart(2, '0')} ${ampm}`,
      value: { minutes, militaryHrs },
    });
  }

  return times;
}

/**
 * getStandardHoursAndPostfix returns hours in standard time
 * (non military time) and its time postfix (AM | PM)
 *
 * @param militaryHrs is 24 hour time
 */
export function getStandardHoursAndPostfix(militaryHrs: number) {
  const ampm = militaryHrs % 24 < 12 ? 'AM' : 'PM';
  let standardHrs = militaryHrs % 12;
  if (standardHrs === 0) {
    standardHrs = 12;
  }

  return {
    ampm,
    hours: standardHrs,
  };
}

export function convertDateToStart(startDate: Date): Start {
  const militaryHrs = startDate.getHours();
  const { hours, ampm } = getStandardHoursAndPostfix(militaryHrs);
  const minutes = startDate.getMinutes();
  const time = {
    label: `${hours}:${String(minutes).padStart(2, '0')} ${ampm} (Requested)`,
    value: {
      hours,
      minutes,
      militaryHrs,
    },
  };

  return { date: new Date(startDate), time };
}

/**
 * Calculates selectable time options based on the day user has selected.
 * There are limits to the earliest time option selectable and to the
 * latest time option selectable based on the time options (`created` and
 * `maxDuration`) returned from the the initial dry run access request.
 */
export function getTimeOptions(
  selectedDate: Date,
  accessRequest: AccessRequest
): TimeOption[] {
  const maxAssumableDate = getMaxAssumableDate(accessRequest);

  // `xxxDatePart` is without the time part, eg: 12/15/2024 09:30:00 -> 12/15/2024
  const createdDatePart = accessRequest.created.toDateString();
  const maxDatePart = maxAssumableDate.toDateString();
  const startDatePart = selectedDate.toDateString();

  // Subtract an hour off the max available time option so user's
  // don't select the shortest time possible.
  const modifiedMaxDurationHrs = maxAssumableDate.getHours() - 1;
  const modifiedMaxDuration = new Date(maxAssumableDate).setHours(
    modifiedMaxDurationHrs
  );

  // Today was the only day available to select.
  // This means there will be both a start limit (earliest time selectable)
  // and a end limit (latest time selectable).
  if (startDatePart === createdDatePart && startDatePart === maxDatePart) {
    return generateTimeDropdown({
      minTimestamp: accessRequest.created.getTime(),
      maxTimestamp: modifiedMaxDuration,
      startDate: selectedDate,
    });
  }
  // User selected the first day among other selectable days.
  // This means there is only a start limit (earliest time selectable)
  // and end is only limited to the last time available for the day (23:59)
  else if (startDatePart === createdDatePart) {
    return generateTimeDropdown({
      minTimestamp: accessRequest.created.getTime(),
      startDate: selectedDate,
    });
  }
  // User selected the last day among other selectable days.
  // This means there is only a end limit (latest time selectable) and
  // start is only limited to the earliest time available for the day (00:00)
  else if (startDatePart === maxDatePart) {
    return generateTimeDropdown({
      maxTimestamp: modifiedMaxDuration,
      startDate: selectedDate,
    });
  }
  // User selected in between the first and last date selectable, so
  // any time options are selectable (00:00 - 23:59)
  else {
    return generateTimeDropdown({ startDate: selectedDate });
  }
}

function getDurationLabel(
  { start, end }: { start: Date; end: Date },
  addMaxLabel = false
) {
  const label = getFormattedDurationTxt({ start, end });

  if (addMaxLabel) {
    return `${label} (Max Duration)`;
  }

  return label;
}

function getMaxDurationOption(
  startDateTime: Date,
  maxDuration: Date
): Option<number> {
  return {
    value: maxDuration.getTime(),
    label: getDurationLabel(
      {
        start: startDateTime,
        end: maxDuration,
      },
      true /* show max duration text */
    ),
  };
}

// Preset hour options for the access duration dropdown.
export const presetHours = [1, 2, 3, 4, 6, 8, 12, 18];
// Preset day options, up to the maximum possible duration, for
// the access duration dropdown. The backend maximum duration
// is two weeks.
export const presetDays = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14];

export type DurationOption = Option<number>;

/**
 * Calculates the selectable access duration options depending
 * on the selected start time. The duration range is starting from the
 * "selected start time" to the request's "max duration".
 *
 * Access duration refers to how long access should last after
 * start date/time.
 */
export function getDurationOptionsFromStartTime(
  selectedDate: Date,
  startTime: TimeOption,
  accessRequest: AccessRequest
): DurationOption[] {
  const startDateTime = getStartDateTime({
    date: selectedDate,
    time: startTime,
  });

  // The setSeconds(0,0) removes the seconds and milliseconds since
  // `startDateTime` is constructed without them. Makes comparing difference
  // in hours ignore the small time difference.
  const createdTimestamp = new Date(accessRequest.created).setSeconds(0, 0);
  const maxDurationTimestamp = new Date(accessRequest.maxDuration).setSeconds(
    0,
    0
  );

  if (
    startDateTime.getTime() >= maxDurationTimestamp ||
    startDateTime.getTime() < createdTimestamp
  ) {
    return [];
  }

  let durationOpts: Option<number>[] = [];

  const totalHoursDiff =
    (maxDurationTimestamp - startDateTime.getTime()) / HourInMs;

  // If there is less than an hour available for access, return
  // it as max duration as the only option.
  if (totalHoursDiff <= 1) {
    durationOpts.push(
      getMaxDurationOption(startDateTime, accessRequest.maxDuration)
    );
    return durationOpts;
  }

  // Add preset hour options up to maximum allowed.
  for (const hour of presetHours) {
    const updatedDateTime = addHours(startDateTime, hour);
    if (isAfter(updatedDateTime, accessRequest.maxDuration)) {
      break;
    }
    durationOpts.push({
      value: updatedDateTime.getTime(),
      label: getDurationLabel({ start: startDateTime, end: updatedDateTime }),
    });
  }

  // Add preset days up to maximum allowed.
  if (totalHoursDiff >= 24) {
    for (const day of presetDays) {
      const updatedEndDate = addDays(startDateTime, day);
      if (isAfter(updatedEndDate, accessRequest.maxDuration)) {
        break;
      }
      durationOpts.push({
        value: updatedEndDate.getTime(),
        label: getDurationLabel({ start: startDateTime, end: updatedEndDate }),
      });
    }
  }

  const lastDurationOption = durationOpts[durationOpts.length - 1];
  if (maxDurationTimestamp > lastDurationOption.value) {
    durationOpts.push(
      getMaxDurationOption(startDateTime, accessRequest.maxDuration)
    );
  } else {
    lastDurationOption.label = `${lastDurationOption.label} (Max Duration)`;
  }

  return durationOpts;
}

export function getDurationOptionIndexClosestToOneWeek(
  durationOptions: DurationOption[],
  startDate: Date
) {
  const oneWeekFromSelectedTime = addWeeks(startDate, 1).getTime();
  const lastDurationIndex = durationOptions.length - 1;

  // Default to the last option, since that is the maxest the user can get.
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

export function getMaxAssumableDate({
  created,
  maxDuration,
}: {
  created: Date;
  maxDuration: Date;
}) {
  let maxAssumableDate = addDays(created, OneWeek);

  // Max duration can be greater than one week.
  // Select the lesser value.
  if (maxAssumableDate.getTime() > maxDuration.getTime()) {
    maxAssumableDate = maxDuration;
  }

  return maxAssumableDate;
}
