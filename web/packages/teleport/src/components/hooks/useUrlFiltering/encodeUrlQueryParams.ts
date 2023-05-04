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

const ADVANCED_SEARCH_PARAM = 'query=';
const SIMPLE_SEARCH_PARAM = 'search=';
const SORT_SEARCH_PARAM = 'sort=';

export function encodeUrlQueryParams(
  pathname: string,
  searchString: string,
  sort: SortType,
  isAdvancedSearch: boolean
) {
  if (!searchString && !sort) {
    return pathname;
  }
  const encodedQuery = encodeURIComponent(searchString);

  const searchParam = isAdvancedSearch
    ? ADVANCED_SEARCH_PARAM
    : SIMPLE_SEARCH_PARAM;

  if (encodedQuery && !sort) {
    return `${pathname}?${searchParam}${encodedQuery}`;
  }

  const sortParam = `${sort.fieldName}:${sort.dir.toLowerCase()}`;

  if (!encodedQuery && sort) {
    return `${pathname}?${SORT_SEARCH_PARAM}${sortParam}`;
  }

  return `${pathname}?${searchParam}${encodedQuery}&${SORT_SEARCH_PARAM}${sortParam}`;
}
