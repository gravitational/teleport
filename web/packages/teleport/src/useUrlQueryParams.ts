/**
 * Copyright 2021 Gravitational, Inc.
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

import { useMemo } from 'react';
import { useLocation, useHistory } from 'react-router';

import { Filter } from 'teleport/types';

// QUERY_PARAM_FILTER is the url query parameter name for filters.
const QUERY_PARAM_FILTER = '?q=';

// FILTER_TYPE_LABEL is the filter identifier name for a label in a filter query.
const FILTER_TYPE_LABEL = 'l=';

export default function useUrlQueryParams() {
  const history = useHistory();
  const { search, pathname } = useLocation();
  const filters = useMemo<Filter[]>(() => getFiltersFromUrl(search), [search]);

  function applyFilters(filters: Filter[]) {
    history.replace(getEncodedUrl(filters, pathname));
  }

  // toggleFilter removes an existing filter from the
  // filters list, else adds new filter to list.
  function toggleFilter(filter: Filter) {
    let modifiedList = [...filters];
    const index = filters.findIndex(
      o => o.name === filter.name && o.value === filter.value
    );

    if (index > -1) {
      // remove the filter
      modifiedList.splice(index, 1);
    } else {
      modifiedList = [...filters, filter];
    }

    applyFilters(modifiedList);
  }

  return {
    filters,
    applyFilters,
    toggleFilter,
  };
}

// getLabelsFromUrl parses the query string
// and returns extracted labels.
function getFiltersFromUrl(search: string): Filter[] {
  if (!search.startsWith(QUERY_PARAM_FILTER)) {
    return [];
  }

  const query = search.substring(QUERY_PARAM_FILTER.length);
  if (!query) {
    return [];
  }

  const queryFilters = query.split('+');
  const filters: Filter[] = [];

  for (let i = 0; i < queryFilters.length; i++) {
    const queryFilter = queryFilters[i];
    if (!queryFilter) {
      continue;
    }

    if (queryFilter.startsWith(FILTER_TYPE_LABEL)) {
      const encodedLabel = queryFilter.substring(FILTER_TYPE_LABEL.length);
      const [encodedName, encodedValue, more] = encodedLabel.split(':');

      // Abort if more than two values were split (malformed label).
      if (more) {
        return [];
      }

      const filter: Filter = {
        name: decodeURIComponent(encodedName ?? ''),
        value: decodeURIComponent(encodedValue ?? ''),
        kind: 'label',
      };

      if (!filter.name && !filter.value) {
        continue;
      }

      filters.push(filter);

      continue;
    }
  }

  return filters;
}

// getEncodedUrl formats and encodes the filters into a
// query format we expect in the URL.
//
// Unencoded delimiters used in query string:
//  - plus (+): used as filter separator, interpreted as AND (&&) operator
//  - colon (:): separates name value pair of a label (ie: country:Spain)
//  - equal (=): used with a filter identifier (ie: `l=`), defines a filter
//
// Format of the query:
// <path>?q=l=<encodedName1>:<encodedValue1>+l=<encodedName2>:<encodedValue2>
function getEncodedUrl(filters: Filter[], pathname = '') {
  if (!filters.length) {
    return pathname;
  }

  const labelFilters = filters
    .map(filter => {
      const encodedName = encodeURIComponent(filter.name);
      const encodedValue = encodeURIComponent(filter.value);

      switch (filter.kind) {
        case 'label':
          return `${FILTER_TYPE_LABEL}${encodedName}:${encodedValue}`;
      }
    })
    .join('+');

  return `${pathname}${QUERY_PARAM_FILTER}${labelFilters}`;
}

export type State = ReturnType<typeof useUrlQueryParams>;
