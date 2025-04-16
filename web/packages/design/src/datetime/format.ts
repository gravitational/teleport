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

/** Converts a UNIX timestamp do the Date object. */
export function unixTimestampToDate(seconds: number): Date {
  return new Date(seconds * 1000);
}

function dateToUtc(date: Date): Date {
  return new Date(date.getTime() + date.getTimezoneOffset() * 60 * 1000);
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
