/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useEffect, useState } from 'react';
import {
  decodeUrlQueryParam,
  ResourceUrlQueryParams,
} from 'teleport/getUrlQueryParams';
import { SortDir } from 'design/DataTable/types';

export default function useServersideSearchPanel(props: Props) {
  const { pathname, params, setParams, replaceHistory } = props;
  const [isInitialLoad, setIsInitialLoad] = useState(true);
  const [searchString, setSearchString] = useState(() => {
    if (params.query) {
      return params.query;
    } else if (params.search) {
      return params.search;
    } else {
      return '';
    }
  });
  const [isAdvancedSearch, setIsAdvancedSearch] = useState(!!params.query);

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
      encodeUrlQueryParams(
        pathname,
        searchString,
        isAdvancedSearch,
        params.sort
      )
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
  }, []);

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
    ...props,
  };
}

const ADVANCED_SEARCH_PARAM = 'query=';
const SIMPLE_SEARCH_PARAM = 'search=';
const SORT_SEARCH_PARAM = 'sort=';

function encodeUrlQueryParams(
  pathname: string,
  searchString: string,
  isAdvancedSearch: boolean,
  sort: SortType
) {
  if (!searchString && !sort) {
    return pathname;
  }
  const encodedQuery = encodeURIComponent(searchString);

  if (encodedQuery && !sort) {
    return `${pathname}?${
      isAdvancedSearch ? ADVANCED_SEARCH_PARAM : SIMPLE_SEARCH_PARAM
    }${encodedQuery}`;
  }

  if (!encodedQuery && sort) {
    return `${pathname}?${`${SORT_SEARCH_PARAM}${
      sort.fieldName
    }:${sort.dir.toLowerCase()}`}`;
  }

  return `${pathname}?${
    isAdvancedSearch ? ADVANCED_SEARCH_PARAM : SIMPLE_SEARCH_PARAM
  }${encodedQuery}&${`${SORT_SEARCH_PARAM}${
    sort.fieldName
  }:${sort.dir.toLowerCase()}`}`;
}

export type Props = {
  pathname: string;
  replaceHistory: (path: string) => void;
  params: ResourceUrlQueryParams;
  setParams: (params: ResourceUrlQueryParams) => void;
  from: number;
  to: number;
  count: number;
};

export type SortType = {
  fieldName: string;
  dir: SortDir;
};

export type State = ReturnType<typeof useServersideSearchPanel>;
