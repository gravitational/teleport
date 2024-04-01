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

import Logger from 'shared/libs/logger';
import cfg from 'shared/config';

const DEFAULT_LOCALE = 'en-US';
const isTest = process.env.NODE_ENV === 'test';

const logger = Logger.create('services/loc');

// displayUnixDate accepts a unix timestamp and returns formatted as 'yyyy-MM-dd'
export function displayUnixDate(seconds: number) {
  // Multiply by 1000 b/c date constructor expects milliseconds.
  const date = new Date(seconds * 1000);

  return displayDate(date);
}

// displayDate accepts a date and returns formatted as 'yyyy-MM-dd'
export function displayDate(date: Date) {
  try {
    if (isTest) {
      return format(dateToUtc(date), cfg.dateFormat);
    }
    return format(date, cfg.dateFormat);
  } catch (err) {
    logger.error('displayDate()', err);
    return 'undefined';
  }
}

// displayShortDate accepts a date and returns formatted as 'MM dd, yyyy'
export function displayShortDate(date: Date) {
  try {
    if (isTest) {
      return format(dateToUtc(date), cfg.shortFormat);
    }
    return format(date, cfg.shortFormat);
  } catch (err) {
    logger.error('displayDate()', err);
    return 'undefined';
  }
}

// displayShortDate accepts a date and returns formatted as 'MM dd, yyyy'
export function displayUnixShortDate(seconds: number) {
  // Multiply by 1000 b/c date constructor expects milliseconds.
  const date = new Date(seconds * 1000);

  return displayShortDate(date);
}

// displayDateTime accepts a date and returns formatted as 'yyyy-MM-dd HH:mm:ss'
export function displayDateTime(date: Date) {
  try {
    if (isTest) {
      return format(dateToUtc(date), cfg.dateTimeFormat);
    }
    return format(date, cfg.dateTimeFormat);
  } catch (err) {
    logger.error('displayDateTime()', err);
    return 'undefined';
  }
}

export function dateToUtc(date: Date) {
  return new Date(date.getTime() + date.getTimezoneOffset() * 60 * 1000);
}

/**
 * Accepts a date and returns the formatted time part of the date.
 * The format depends on the browser and system settings locale,
 * eg: if locale was `en-US` the returned value will be say `4:00 PM`.
 *
 * During tests, the locale will always default to `en-US`.
 */
export function dateTimeShortFormat(date: Date) {
  const locale = isTest ? DEFAULT_LOCALE : undefined;
  return new Intl.DateTimeFormat(locale, { timeStyle: 'short' }).format(date);
}
