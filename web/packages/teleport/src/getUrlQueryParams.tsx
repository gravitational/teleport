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

import { SortType } from 'design/DataTable/types';

export default function getResourceUrlQueryParams(
  searchPath: string
): ResourceUrlQueryParams {
  const searchParams = new URLSearchParams(searchPath);
  const query = searchParams.get('query');
  const search = searchParams.get('search');
  const sort = searchParams.get('sort');

  // Converts the "fieldname:dir" format into {fieldName: "", dir: ""}
  const processedSortParam = sort
    ? ({
        fieldName: sort.split(':')[0],
        dir: sort.split(':')[1]?.toUpperCase() || 'ASC',
      } as SortType)
    : null;

  return {
    query,
    search,
    // Conditionally adds the sort field based on whether it exists or not
    ...(!!processedSortParam && { sort: processedSortParam }),
  };
}

export function decodeUrlQueryParam(param: string) {
  // Prevents URI malformed error by replacing lone % with %25
  const decodedQuery = decodeURIComponent(
    param.replace(/%(?![0-9][0-9a-fA-F]+)/g, '%25')
  );

  return decodedQuery;
}

export type ResourceUrlQueryParams = {
  query?: string;
  search?: string;
  sort?: SortType;
};
