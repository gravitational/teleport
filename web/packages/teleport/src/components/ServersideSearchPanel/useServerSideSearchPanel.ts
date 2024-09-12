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

import { useEffect, useState } from 'react';

import { ResourceFilter } from 'teleport/services/agents';

import {
  decodeUrlQueryParam,
  encodeUrlQueryParams,
} from 'teleport/components/hooks/useUrlFiltering';

export default function useServersideSearchPanel({
  pathname,
  params,
  setParams,
  replaceHistory,
}: HookProps) {
  const [searchString, setSearchString] = useState('');
  const [isAdvancedSearch, setIsAdvancedSearch] = useState(false);
  const [isInitialLoad, setIsInitialLoad] = useState(true);

  function onSubmitSearch(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    submitSearch();
  }

  function submitSearch() {
    if (isAdvancedSearch) {
      setParams({
        ...params,
        search: null,
        query: searchString,
      });
    } else {
      setParams({
        ...params,
        query: null,
        search: searchString,
      });
    }
    replaceHistory(
      encodeUrlQueryParams({
        pathname,
        searchString,
        sort: params.sort,
        kinds: params.kinds,
        isAdvancedSearch,
        pinnedOnly: params.pinnedOnly,
      })
    );
  }

  // Populate search bar with existing query
  useEffect(() => {
    if (params.query) {
      setIsAdvancedSearch(true);
      setSearchString(decodeUrlQueryParam(params.query));
    } else if (params.search) {
      setIsAdvancedSearch(false);
      setSearchString(decodeUrlQueryParam(params.search));
    }
  }, [params.query, params.search]);

  useEffect(() => {
    if (!isInitialLoad) {
      submitSearch();
    }
    setIsInitialLoad(false);
  }, [params.sort]);

  return {
    searchString,
    setSearchString,
    isAdvancedSearch,
    setIsAdvancedSearch,
    onSubmitSearch,
  };
}

export type HookProps = {
  pathname: string;
  replaceHistory: (path: string) => void;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
};

export type SearchPanelState = ReturnType<typeof useServersideSearchPanel>;
