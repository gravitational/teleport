/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { format } from 'date-fns';

import Logger from 'shared/libs/logger';
import cfg from 'shared/config';

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
