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

import { useMemo, useState } from 'react';
import { useLocation } from 'react-router';

import { SortType } from 'design/DataTable/types';
import { IncludedResourceMode } from 'shared/components/UnifiedResources';
import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';

import { ResourceFilter, ResourceLabel } from 'teleport/services/agents';
import history from 'teleport/services/history';

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

type URLResourceFilter = Omit<ResourceFilter, 'includedResourceMode'>;

export function useUrlFiltering(
  initialParams: URLResourceFilter,
  includedResourceMode?: IncludedResourceMode
): UrlFilteringState {
  const { search, pathname } = useLocation();

  function replaceHistory(path: string) {
    history.replace(path);
  }

  function setSort(sort: SortType) {
    replaceHistory(
      encodeUrlQueryParams({
        pathname,
        searchString: params.search || params.query,
        sort: { ...params.sort, ...sort },
        kinds: params.kinds,
        isAdvancedSearch: !!params.query,
        pinnedOnly: params.pinnedOnly,
      })
    );
  }

  const [initialParamsState] = useState(initialParams);
  const params = useMemo(() => {
    const urlParams = getResourceUrlQueryParams(search);
    return {
      ...initialParamsState,
      ...urlParams,
      includedResourceMode,
      pinnedOnly:
        urlParams.pinnedOnly !== undefined
          ? urlParams.pinnedOnly
          : initialParamsState.pinnedOnly,
    };
  }, [search, includedResourceMode]);

  function setParams(newParams: URLResourceFilter) {
    replaceHistory(
      encodeUrlQueryParams({
        pathname,
        searchString: newParams.search || newParams.query,
        sort: newParams.sort,
        kinds: newParams.kinds,
        isAdvancedSearch: !!newParams.query,
        pinnedOnly: newParams.pinnedOnly,
      })
    );
  }

  const onLabelClick = (label: ResourceLabel) => {
    const queryAfterLabelClick = makeAdvancedSearchQueryForLabel(label, params);

    replaceHistory(
      encodeUrlQueryParams({
        pathname,
        searchString: queryAfterLabelClick,
        sort: params.sort,
        kinds: params.kinds,
        isAdvancedSearch: true,
        pinnedOnly: params.pinnedOnly,
      })
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
    pinnedOnly:
      pinnedOnly === 'true' ? true : pinnedOnly === 'false' ? false : undefined,
  };
}
