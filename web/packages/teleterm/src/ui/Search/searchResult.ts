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

import type { Cluster } from 'teleterm/services/tshd/types';
import type * as resourcesServiceTypes from 'teleterm/ui/services/resources';
import type { DocumentClusterResourceKind } from 'teleterm/ui/services/workspacesService';
import type { ClusterUri, DocumentUri } from 'teleterm/ui/uri';

type ResourceSearchResultBase<
  Result extends resourcesServiceTypes.SearchResult,
> = Result & {
  labelMatches: LabelMatch[];
  resourceMatches: ResourceMatch<Result['kind']>[];
  score: number;
};

export type ResourceTypeFilter = DocumentClusterResourceKind;

export type SearchResultServer =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultServer>;
export type SearchResultDatabase =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultDatabase>;
export type SearchResultKube =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultKube>;
export type SearchResultApp =
  ResourceSearchResultBase<resourcesServiceTypes.SearchResultApp>;
export type SearchResultCluster = {
  kind: 'cluster-filter';
  resource: Cluster;
  nameMatch: string;
  score: number;
};
export type SearchResultResourceType = {
  kind: 'resource-type-filter';
  resource: ResourceTypeFilter;
  nameMatch: string;
  score: number;
};
export type DisplayResults = {
  kind: 'display-results';
  value: string;
  resourceKinds: DocumentClusterResourceKind[];
  clusterUri: ClusterUri;
  documentUri: DocumentUri | undefined;
};

// TODO(gzdunek): find a better name.
// `ResourcesService` exports `SearchResult` which is then usually imported as `ResourceSearchResult`.
// Having these two thing named almost the same is confusing.
export type ResourceSearchResult =
  | SearchResultServer
  | SearchResultDatabase
  | SearchResultKube
  | SearchResultApp;

export type FilterSearchResult = SearchResultResourceType | SearchResultCluster;

export type SearchResult =
  | ResourceSearchResult
  | FilterSearchResult
  | DisplayResults;

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
  [Kind in ResourceSearchResult['kind']]: keyof resourcesServiceTypes.SearchResultResource<Kind>;
} = {
  server: 'hostname',
  database: 'name',
  kube: 'name',
  app: 'name',
} as const;

// The usage of Exclude here is a workaround to make sure that the fields in the array point only to
// fields of string type.
export const searchableFields: {
  [Kind in ResourceSearchResult['kind']]: ReadonlyArray<
    Exclude<keyof resourcesServiceTypes.SearchResultResource<Kind>, 'labels'>
  >;
} = {
  server: ['name', 'hostname', 'addr'],
  database: ['name', 'desc', 'protocol', 'type'],
  kube: ['name'],
  // Right now, friendlyName is set only for Okta apps (api/types/resource.go).
  // The friendly name is constructed *after* fetching apps, but since it is
  // made from the value of a label, the server-side search can find it.
  app: ['name', 'friendlyName', 'desc', 'addrWithProtocol'],
} as const;

export interface ResourceTypeSearchFilter {
  filter: 'resource-type';
  resourceType: ResourceTypeFilter;
}

export interface ClusterSearchFilter {
  filter: 'cluster';
  clusterUri: ClusterUri;
}

export type SearchFilter = ResourceTypeSearchFilter | ClusterSearchFilter;

export function isResourceTypeSearchFilter(
  searchFilter: SearchFilter
): searchFilter is ResourceTypeSearchFilter {
  return searchFilter.filter === 'resource-type';
}

export function isClusterSearchFilter(
  searchFilter: SearchFilter
): searchFilter is ClusterSearchFilter {
  return searchFilter.filter === 'cluster';
}
