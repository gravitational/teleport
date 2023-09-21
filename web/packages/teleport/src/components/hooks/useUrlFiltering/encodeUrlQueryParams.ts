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

import { SortType } from 'design/DataTable/types';

export function encodeUrlQueryParams(
  pathname: string,
  searchString: string,
  sort: SortType | null,
  kinds: string[] | null,
  isAdvancedSearch: boolean
) {
  const urlParams = new URLSearchParams();

  if (searchString) {
    urlParams.append(isAdvancedSearch ? 'query' : 'search', searchString);
  }

  if (sort) {
    urlParams.append('sort', `${sort.fieldName}:${sort.dir.toLowerCase()}`);
  }

  if (kinds) {
    for (const kind of kinds) {
      urlParams.append('kinds', kind);
    }
  }

  const encodedParams = urlParams.toString();

  return encodedParams ? `${pathname}?${encodedParams}` : pathname;
}
