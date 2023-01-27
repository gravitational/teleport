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

import { useState } from 'react';
import { useLocation } from 'react-router';
import { SortType } from 'design/DataTable/types';
import history from 'teleport/services/history';
import { AgentLabel } from 'teleport/services/agents';
import getQueryParams from './getQueryParams';
import labelClick from './labelClick';

export default function useUrlFiltering(initialSort: SortType) {
  const { search, pathname } = useLocation();
  const [params, setParams] = useState<ResourceUrlQueryParams>({
    sort: initialSort,
    ...getQueryParams(search),
  });

  function replaceHistory(path: string) {
    history.replace(path);
  }

  function setSort(sort: SortType) {
    setParams({ ...params, sort });
  }

  const onLabelClick = (label: AgentLabel) =>
    labelClick(label, params, setParams, pathname, replaceHistory);

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

export type ResourceUrlQueryParams = {
  query?: string;
  search?: string;
  sort?: SortType;
};
