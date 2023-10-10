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

import { format } from 'date-fns';

export const dateFormat = 'MM/dd/yyyy';

export function getFormattedDate(d: Date) {
  if (!d || isNaN(d.getTime())) {
    return '';
  }

  // The zero value for golang Date comes back as "0001-01-01T00:00:00Z"
  // which is January 1, year 1.
  // JS zero date is January 1, 1970  which is "greater" than
  // golang's zero value. So it's safe to assume that backend Dates
  // that are less than JS's zero value means the date was not set.
  const zeroDate = new Date(0);
  const thisDate = new Date(d);
  if (thisDate <= zeroDate) {
    return '';
  }

  return format(thisDate, dateFormat);
}
