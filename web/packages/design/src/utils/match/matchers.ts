/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { displayDate, displayDateTime } from 'shared/services/loc';

import { MatchCallback } from './match';

export function dateMatcher<T>(
  datePropNames: (keyof T & string)[]
): MatchCallback<T> {
  return (targetValue, searchValue, propName) => {
    if (datePropNames.includes(propName)) {
      return displayDate(new Date(targetValue))
        .toLocaleUpperCase()
        .includes(searchValue);
    }
  };
}

export function dateTimeMatcher<T>(
  dateTimePropNames: (keyof T & string)[]
): MatchCallback<T> {
  return (targetValue, searchValue, propName) => {
    if (dateTimePropNames.includes(propName)) {
      return displayDateTime(new Date(targetValue))
        .toLocaleUpperCase()
        .includes(searchValue);
    }
  };
}
