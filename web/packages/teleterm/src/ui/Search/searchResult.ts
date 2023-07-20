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

import type { ClusterUri } from 'teleterm/ui/uri';
import type { Cluster } from 'teleterm/services/tshd/types';

import type * as resourcesServiceTypes from 'teleterm/ui/services/resources';
import type { SearchResultResource } from 'teleterm/ui/services/resources';

export { SearchResultResource };

type ResourceSearchResultBase<
  Result extends resourcesServiceTypes.SearchResult,
> = Result & {
  labelMatches: LabelMatch[];
  resourceMatches: ResourceMatch<Result['kind']>[];
  score: number;
};

export type SearchResultServer =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultServer>;
export type SearchResultDatabase =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultDatabase>;
export type SearchResultKube =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultKube>;
export type SearchResultCluster = {
  kind: 'cluster-filter';
  resource: Cluster;
  nameMatch: string;
  score: number;
};
export type SearchResultResourceType = {
  kind: 'resource-type-filter';
  resource: 'kubes' | 'servers' | 'databases';
  nameMatch: string;
  score: number;
};

// TODO(gzdunek): find a better name.
// `ResourcesService` exports `SearchResult` which is then usually imported as `ResourceSearchResult`.
// Having these two thing named almost the same is confusing.
export type ResourceSearchResult =
  | SearchResultServer
  | SearchResultDatabase
  | SearchResultKube;

export type FilterSearchResult = SearchResultResourceType | SearchResultCluster;

export type SearchResult = ResourceSearchResult | FilterSearchResult;

export type LabelMatch = {
  kind: 'label-name' | 'label-value';
  labelName: string;
  searchTerm: string;
  // Individual score of this label match; how much it contributes to the total score.
  score: number;
};

export type ResourceMatch<Kind extends ResourceSearchResult['kind']> = {
  field: (typeof searchableFields)[Kind][number];
  searchTerm: string;
};

/**
 * mainResourceName returns the main identifier for the given resource displayed in the UI.
 */
export function mainResourceName(searchResult: ResourceSearchResult): string {
  return searchResult.resource[mainResourceField[searchResult.kind]];
}

export const mainResourceField: {
  [Kind in ResourceSearchResult['kind']]: keyof SearchResultResource<Kind>;
} = {
  server: 'hostname',
  database: 'name',
  kube: 'name',
} as const;

// The usage of Exclude here is a workaround to make sure that the fields in the array point only to
// fields of string type.
export const searchableFields: {
  [Kind in ResourceSearchResult['kind']]: ReadonlyArray<
    Exclude<keyof SearchResultResource<Kind>, 'labelsList'>
  >;
} = {
  server: ['name', 'hostname', 'addr'],
  database: ['name', 'desc', 'protocol', 'type'],
  kube: ['name'],
} as const;

export interface ResourceTypeSearchFilter {
  filter: 'resource-type';
  resourceType: 'kubes' | 'servers' | 'databases';
}

export interface ClusterSearchFilter {
  filter: 'cluster';
  clusterUri: ClusterUri;
}

export type SearchFilter = ResourceTypeSearchFilter | ClusterSearchFilter;
