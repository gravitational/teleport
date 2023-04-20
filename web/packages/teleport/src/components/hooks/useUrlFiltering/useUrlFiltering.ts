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

import { useState } from 'react';
import { useLocation } from 'react-router';
import { SortType } from 'design/DataTable/types';

import history from 'teleport/services/history';
import { AgentFilter, AgentLabel } from 'teleport/services/agents';

import { encodeUrlQueryParams } from './encodeUrlQueryParams';

export function useUrlFiltering(initialSort: SortType) {
  const { search, pathname } = useLocation();
  const [params, setParams] = useState<AgentFilter>({
    sort: initialSort,
    ...getResourceUrlQueryParams(search),
  });

  function replaceHistory(path: string) {
    history.replace(path);
  }

  function setSort(sort: SortType) {
    setParams({ ...params, sort });
  }

  const onLabelClick = (label: AgentLabel) => {
    const queryAfterLabelClick = labelClickQuery(label, params);

    setParams({ ...params, search: '', query: queryAfterLabelClick });
    replaceHistory(
      encodeUrlQueryParams(pathname, queryAfterLabelClick, params.sort, true)
    );
  };

  const isSearchEmpty = !params?.query && !params?.search;

  return {
    isSearchEmpty,
    params,
    setParams,
    pathname,
    setSort,
    onLabelClick,
    replaceHistory,
    search,
  };
}

export default function getResourceUrlQueryParams(
  searchPath: string
): AgentFilter {
  const searchParams = new URLSearchParams(searchPath);
  const query = searchParams.get('query');
  const search = searchParams.get('search');
  const sort = searchParams.get('sort');

  const sortParam = sort ? sort.split(':') : null;

  // Converts the "fieldname:dir" format into {fieldName: "", dir: ""}
  const processedSortParam = sortParam
    ? ({
        fieldName: sortParam[0],
        dir: sortParam[1]?.toUpperCase() || 'ASC',
      } as SortType)
    : null;

  return {
    query,
    search,
    // Conditionally adds the sort field based on whether it exists or not
    ...(!!processedSortParam && { sort: processedSortParam }),
  };
}

function labelClickQuery(label: AgentLabel, params: AgentFilter) {
  const queryParts: string[] = [];

  // Add existing query
  if (params.query) {
    queryParts.push(params.query);
  }

  // If there is an existing simple search, convert it to predicate language and add it
  if (params.search) {
    queryParts.push(`search("${params.search}")`);
  }

  const labelQuery = `labels["${label.name}"] == "${label.value}"`;
  queryParts.push(labelQuery);

  const finalQuery = queryParts.join(' && ');

  return finalQuery;
}
