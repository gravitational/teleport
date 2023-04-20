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

import { pluralize } from './text';

export function getDurationText(hrs: number, mins: number, secs: number) {
  if (!hrs && !mins) {
    return `${secs} secs`;
  }

  const hrText = pluralize(hrs, 'hr');
  const minText = pluralize(mins, 'min');

  if (!hrs) {
    return `${mins} ${minText}`;
  }

  if (hrs && !mins) {
    return `${hrs} ${hrText}`;
  }

  return `${hrs} ${hrText} and ${mins} ${minText}`;
}
