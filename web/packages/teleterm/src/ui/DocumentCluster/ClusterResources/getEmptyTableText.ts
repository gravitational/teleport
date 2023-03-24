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

import { AttemptStatus } from 'shared/hooks/useAsync';

/**
 *  `getEmptyTableText` returns text to be used in an async resource table
 *
 *  @example
 *  // Successfully fetched with zero results returned
 *  getEmptyTableText(fetchAttempt.status, "servers"); // "No servers found"
 *
 *  @param status - AttemptStatus from a useAsync request
 *  @param pluralResourceNoun - String that represents the plural of a resource, i.e. "servers", "databases"
 */
export function getEmptyTableText(
  status: AttemptStatus,
  pluralResourceNoun: string
) {
  switch (status) {
    case 'error':
      return `Failed to fetch ${pluralResourceNoun}.`;
    case '':
      return 'Searching…';
    case 'processing':
      return 'Searching…';
    case 'success':
      return `No ${pluralResourceNoun} found.`;
  }
}
