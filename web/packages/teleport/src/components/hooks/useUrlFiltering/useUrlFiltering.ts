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

import { useState } from 'react';
import { useLocation } from 'react-router';
import { SortType } from 'design/DataTable/types';

import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';

import history from 'teleport/services/history';
import { ResourceFilter, ResourceLabel } from 'teleport/services/agents';

import { encodeUrlQueryParams } from './encodeUrlQueryParams';

export interface UrlFilteringState {
  isSearchEmpty: boolean;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  pathname: string;
  setSort: (sort: SortType) => void;
  onLabelClick: (label: ResourceLabel) => void;
  replaceHistory: (path: string) => void;
  search: string;
}

export function useUrlFiltering(
  initialParams: Partial<ResourceFilter>
): UrlFilteringState {
  const { search, pathname } = useLocation();
  const [params, setParams] = useState<ResourceFilter>({
    ...initialParams,
    ...getResourceUrlQueryParams(search),
  });

  function replaceHistory(path: string) {
    history.replace(path);
  }

  function setSort(sort: SortType) {
    setParams({ ...params, sort });
  }

  const onLabelClick = (label: ResourceLabel) => {
    const queryAfterLabelClick = makeAdvancedSearchQueryForLabel(label, params);

    setParams({ ...params, search: '', query: queryAfterLabelClick });
    replaceHistory(
      encodeUrlQueryParams(
        pathname,
        queryAfterLabelClick,
        params.sort,
        params.kinds,
        true /*isAdvancedSearch*/,
        params.pinnedOnly
      )
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
): ResourceFilter {
  const searchParams = new URLSearchParams(searchPath);
  const query = searchParams.get('query');
  const search = searchParams.get('search');
  const pinnedOnly = searchParams.get('pinnedOnly');
  const sort = searchParams.get('sort');
  const kinds = searchParams.has('kinds') ? searchParams.getAll('kinds') : null;

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
    kinds,
    // Conditionally adds the sort field based on whether it exists or not
    ...(!!processedSortParam && { sort: processedSortParam }),
    // Conditionally adds the pinnedResources field based on whether its true or not
    ...(pinnedOnly === 'true' && { pinnedOnly: true }),
  };
}
