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

import { format } from 'date-fns';

import { Logger } from 'design/logger';

import {
  DATE_FORMAT,
  DATE_TIME_FORMAT,
  DATE_WITH_PREFIXED_TIME_FORMAT,
  SHORT_DATE_FORMAT,
} from './constants';

const DEFAULT_LOCALE = 'en-US';
const isTest = process.env.NODE_ENV === 'test';

const logger = new Logger('datetime/format');

/** Accepts a date and returns formatted as 'yyyy-MM-dd' */
export function displayDate(date: Date): string {
  try {
    if (isTest) {
      return format(dateToUtc(date), DATE_FORMAT);
    }
    return format(date, DATE_FORMAT);
  } catch (err) {
    logger.error('Could not format date', err);
    return 'undefined';
  }
}

/** Accepts a date and returns formatted as 'MM dd, yyyy'. */
export function displayShortDate(date: Date): string {
  try {
    if (isTest) {
      return format(dateToUtc(date), SHORT_DATE_FORMAT);
    }
    return format(date, SHORT_DATE_FORMAT);
  } catch (err) {
    logger.error('Could not format date', err);
    return 'undefined';
  }
}
/** Accepts a date and returns formatted as 'yyyy-MM-dd HH:mm:ss'. */
export function displayDateTime(date: Date): string {
  try {
    if (isTest) {
      return format(dateToUtc(date), DATE_TIME_FORMAT);
    }
    return format(date, DATE_TIME_FORMAT);
  } catch (err) {
    logger.error('Could not format date', err);
    return 'undefined';
  }
}

/**
 * Accepts a date and returns formatted as `LL/dd/yyyy 'at' h:mma`.
 * @TODO Consider removing this format https://github.com/gravitational/teleport/issues/39326.
 * */
export function displayDateWithPrefixedTime(date: Date): string {
  try {
    if (isTest) {
      return format(dateToUtc(date), DATE_WITH_PREFIXED_TIME_FORMAT);
    }
    return format(date, DATE_WITH_PREFIXED_TIME_FORMAT);
  } catch (err) {
    logger.error('Could not format date', err);
    return 'undefined';
  }
}

function dateToUtc(date: Date): Date {
  return new Date(date.getTime() + date.getTimezoneOffset() * 60 * 1000);
}

/**
 * Accepts a date and returns the formatted date part of the date.
 * eg: if locale was `en-US` the returned value will be say `Oct 8, 2026`.
 *
 * During tests, the locale will always default to `en-US`.
 */
export function dateMediumFormat(date: Date): string {
  const locale = isTest ? DEFAULT_LOCALE : undefined;
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(date);
}

/**
 * Accepts a date and returns the formatted time part of the date.
 * The format depends on the browser and system settings locale,
 * eg: if locale was `en-US` the returned value will be say `4:00 PM`.
 *
 * During tests, the locale will always default to `en-US`.
 */
export function dateTimeShortFormat(date: Date): string {
  const locale = isTest ? DEFAULT_LOCALE : undefined;
  return new Intl.DateTimeFormat(locale, { timeStyle: 'short' }).format(date);
}

/**
 * Accepts a date and returns the formatted date part of the date in numeric
 * form. eg: if locale was `en-US` the returned value will be say `10/30/2026`.
 *
 * During tests, the locale will always default to `en-US`.
 */
function dateNumericFormat(date: Date): string {
  const locale = isTest ? DEFAULT_LOCALE : undefined;
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date);
}

export function formatTimeZoneOffset(date: Date, timeZone?: string): string {
  const offset = new Intl.DateTimeFormat(DEFAULT_LOCALE, {
    timeZone,
    timeZoneName: 'shortOffset',
  })
    .formatToParts(date)
    .find(part => part.type === 'timeZoneName')?.value;

  return offset === 'GMT' ? 'UTC+0' : offset?.replace('GMT', 'UTC') || '';
}

/**
 * Accepts a date and returns the short name of its time zone,
 * eg: `MDT` or `UTC`.
 */
export function formatTimeZoneAbbreviation(
  date: Date,
  timeZone?: string
): string {
  return (
    new Intl.DateTimeFormat(DEFAULT_LOCALE, {
      timeZone,
      timeZoneName: 'short',
    })
      .formatToParts(date)
      .find(part => part.type === 'timeZoneName')?.value || ''
  );
}

/**
 * Accepts a date and returns it with its time zone in parentheses,
 * eg: `Oct 30, 2026 at 4:00 PM (MDT)`.
 */
export function displayDateTimeWithTimeZone(date: Date): string {
  return `${dateMediumFormat(date)} at ${dateTimeShortFormat(date)} (${formatTimeZoneAbbreviation(date)})`;
}

/**
 * Accepts a date and returns it with a numeric date part and a bare time zone,
 * eg: `10/30/2026 at 4:00 PM MDT`.
 */
export function displayNumericDateTimeWithTimeZone(date: Date): string {
  return `${dateNumericFormat(date)} at ${dateTimeShortFormat(date)} ${formatTimeZoneAbbreviation(date)}`;
}
