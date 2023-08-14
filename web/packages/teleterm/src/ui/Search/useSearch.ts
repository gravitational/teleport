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

import { useCallback } from 'react';

import { assertUnreachable } from 'teleterm/ui/utils';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import {
  ClusterSearchFilter,
  ResourceTypeSearchFilter,
  SearchFilter,
  LabelMatch,
  mainResourceField,
  mainResourceName,
  ResourceMatch,
  searchableFields,
  ResourceSearchResult,
  FilterSearchResult,
} from './searchResult';

import type * as resourcesServiceTypes from 'teleterm/ui/services/resources';

export type CrossClusterResourceSearchResult = {
  results: resourcesServiceTypes.SearchResult[];
  errors: resourcesServiceTypes.ResourceSearchError[];
  search: string;
};

/**
 * useResourceSearch returns a function which searches for the given list of space-separated keywords across
 * all root and leaf clusters that the user is currently logged in to.
 *
 * It does so by issuing a separate request for each resource type to each cluster. It fails if any
 * of those requests fail.
 */
export function useResourceSearch() {
  const { clustersService, resourcesService } = useAppContext();

  return useCallback(
    async (
      search: string,
      filters: SearchFilter[]
    ): Promise<CrossClusterResourceSearchResult> => {
      const searchMode = getResourceSearchMode(search, filters);
      let limit = 100;

      switch (searchMode) {
        // useResourceSearch has to return _something_ even when we don't want to perform a search.
        // Imagine this scenario:
        //
        // 1. The user types in 'dat' into the search bar.
        // 2. The search bar immediately returns filters and it starts a resource search for 'dat'.
        // 3. The user selects the database filter before the backend response comes back.
        //
        // The request for 'dat' that was in flight needs to be canceled by useAsync somehow. We can
        // do that by calling useResourceSearch again, even with empty input.
        case 'no-search': {
          return { results: [], errors: [], search };
        }
        case 'preview': {
          // In preview mode we know that the user didn't specify any search terms. So instead of
          // fetching all 100 resources for each request, we fetch only a bunch of them to show
          // example results in the UI.
          limit = 5;
          break;
        }
        case 'full-search': {
          // noop, limit remains at 100.
          break;
        }
        default: {
          assertUnreachable(searchMode);
        }
      }

      const clusterSearchFilter = filters.find(
        s => s.filter === 'cluster'
      ) as ClusterSearchFilter;
      const resourceTypeSearchFilter = filters.find(
        s => s.filter === 'resource-type'
      ) as ResourceTypeSearchFilter;

      const connectedClusters = clustersService
        .getClusters()
        .filter(c => c.connected);
      const clustersToSearch = clusterSearchFilter
        ? connectedClusters.filter(
            c => clusterSearchFilter.clusterUri === c.uri
          )
        : connectedClusters;

      // ResourcesService.searchResources uses Promise.allSettled so the returned promise will never
      // get rejected.
      const promiseResults = (
        await Promise.all(
          clustersToSearch.map(cluster =>
            resourcesService.searchResources({
              clusterUri: cluster.uri,
              search,
              filter: resourceTypeSearchFilter,
              limit,
            })
          )
        )
      ).flat();

      const results: resourcesServiceTypes.SearchResult[] = [];
      const errors: resourcesServiceTypes.ResourceSearchError[] = [];

      for (const promiseResult of promiseResults) {
        switch (promiseResult.status) {
          case 'fulfilled': {
            results.push(...promiseResult.value);
            break;
          }
          case 'rejected': {
            errors.push(promiseResult.reason);
            break;
          }
        }
      }

      return { results, errors, search };
    },
    [clustersService, resourcesService]
  );
}

/**
 * `useFilterSearch` returns a function which searches for clusters or resource types,
 * which are later used to narrow down the requests in `useResourceSearch`.
 */
export function useFilterSearch() {
  const { clustersService, workspacesService } = useAppContext();

  return useCallback(
    (search: string, filters: SearchFilter[]): FilterSearchResult[] => {
      const getClusters = () => {
        let clusters = clustersService.getClusters();
        // Cluster filter should not be visible if there is only one cluster
        if (clusters.length === 1) {
          return [];
        }
        if (search) {
          clusters = clusters.filter(cluster =>
            cluster.name
              .toLocaleLowerCase()
              .includes(search.toLocaleLowerCase())
          );
        }
        return clusters.map(cluster => {
          let score = getLengthScore(search, cluster.name);
          if (
            cluster.uri ===
            workspacesService.getActiveWorkspace()?.localClusterUri
          ) {
            // put the active cluster first (only when there is a match, otherwise score is 0)
            score *= 3;
          }
          return {
            kind: 'cluster-filter' as const,
            resource: cluster,
            nameMatch: search,
            score,
          };
        });
      };
      const getResourceType = () => {
        let resourceTypes = [
          'servers' as const,
          'databases' as const,
          'kubes' as const,
        ];
        if (search) {
          resourceTypes = resourceTypes.filter(resourceType =>
            resourceType.toLowerCase().includes(search.toLowerCase())
          );
        }
        return resourceTypes.map(resourceType => ({
          kind: 'resource-type-filter' as const,
          resource: resourceType,
          nameMatch: search,
          score: getLengthScore(search, resourceType),
        }));
      };

      const shouldReturnClusters = !filters.some(r => r.filter === 'cluster');
      const shouldReturnResourceTypes = !filters.some(
        r => r.filter === 'resource-type'
      );

      const results = [
        shouldReturnResourceTypes && getResourceType(),
        shouldReturnClusters && getClusters(),
      ]
        .filter(Boolean)
        .flat()
        .sort((a, b) => {
          // Highest score first.
          return b.score - a.score;
        });

      return results;
    },
    [clustersService, workspacesService]
  );
}

/** Sorts and then returns top 10 results. */
export function rankResults(
  searchResults: resourcesServiceTypes.SearchResult[],
  search: string
): ResourceSearchResult[] {
  const terms = search
    .split(' ')
    .filter(Boolean)
    // We have to match the implementation of the search algorithm as closely as possible. It uses
    // strings.ToLower from Go which unfortunately doesn't have a good equivalent in JavaScript.
    //
    // strings.ToLower uses some kind of a universal map for lowercasing non-ASCII characters such
    // as the Turkish İ. JavaScript doesn't have such a function, possibly because it's not possible
    // to have universal case mapping. [1]
    //
    // The closest thing that JS has is toLocaleLowerCase. Since we don't know what locale the
    // search string uses, we let the runtime figure it out based on the system settings.
    // The assumption is that if someone has a resource with e.g. Turkish characters, their system
    // is set to the appropriate locale and the search results will be properly scored.
    //
    // Highlighting will have problems with some non-ASCII characters anyway because the library we
    // use for highlighting uses a regex with the i flag underneath.
    //
    // [1] https://web.archive.org/web/20190113111936/https://blogs.msdn.microsoft.com/oldnewthing/20030905-00/?p=42643
    .map(term => term.toLocaleLowerCase());
  const collator = new Intl.Collator();

  return searchResults
    .map(searchResult => calculateScore(populateMatches(searchResult, terms)))
    .sort(
      (a, b) =>
        // Highest score first.
        b.score - a.score ||
        collator.compare(mainResourceName(a), mainResourceName(b))
    )
    .slice(0, 10);
}

function populateMatches(
  searchResult: resourcesServiceTypes.SearchResult,
  terms: string[]
): ResourceSearchResult {
  const labelMatches: LabelMatch[] = [];
  const resourceMatches: ResourceMatch<ResourceSearchResult['kind']>[] = [];

  terms.forEach(term => {
    searchResult.resource.labelsList.forEach(label => {
      // indexOf is faster on Chrome than includes or regex.
      // https://jsbench.me/b7lf9kvrux/1
      const nameIndex = label.name.toLocaleLowerCase().indexOf(term);
      const valueIndex = label.value.toLocaleLowerCase().indexOf(term);

      if (nameIndex >= 0) {
        labelMatches.push({
          kind: 'label-name',
          labelName: label.name,
          searchTerm: term,
          score: 0,
        });
      }

      if (valueIndex >= 0) {
        labelMatches.push({
          kind: 'label-value',
          labelName: label.name,
          searchTerm: term,
          score: 0,
        });
      }
    });

    searchableFields[searchResult.kind].forEach(field => {
      // `String` here is just to satisfy the compiler.
      const index = searchResult.resource[field]
        .toLocaleLowerCase()
        .indexOf(term);

      if (index >= 0) {
        resourceMatches.push({
          field,
          searchTerm: term,
        });
      }
    });
  });

  return { ...searchResult, labelMatches, resourceMatches, score: 0 };
}

// TODO(ravicious): Extract the scoring logic to a function to better illustrate different weight
// for different matches.
function calculateScore(
  searchResult: ResourceSearchResult
): ResourceSearchResult {
  let searchResultScore = 0;

  const labelMatches = searchResult.labelMatches.map(match => {
    const label = searchResult.resource.labelsList.find(
      label => label.name === match.labelName
    );
    let matchedValue: string;

    switch (match.kind) {
      case 'label-name': {
        matchedValue = label.name;
        break;
      }
      case 'label-value': {
        matchedValue = label.value;
        break;
      }
      default: {
        assertUnreachable(match.kind);
      }
    }

    const score = getLengthScore(match.searchTerm, matchedValue);
    searchResultScore += score;

    return { ...match, score };
  });

  for (const match of searchResult.resourceMatches) {
    const { searchTerm } = match;
    const field = searchResult.resource[match.field];
    const isMainField = mainResourceField[searchResult.kind] === match.field;
    const weight = isMainField ? 4 : 2;

    const resourceMatchScore = getLengthScore(searchTerm, field) * weight;
    searchResultScore += resourceMatchScore;
  }

  return { ...searchResult, labelMatches, score: searchResultScore };
}

type ResourceSearchMode = 'no-search' | 'preview' | 'full-search';

function getResourceSearchMode(
  search: string,
  filters: SearchFilter[]
): ResourceSearchMode {
  // Trim the search to avoid sending requests with limit set to 100 if the user just pressed some
  // spaces.
  const trimmedSearch = search.trim();

  if (!trimmedSearch) {
    // The preview should be fetched only when at least one filter is selected. Otherwise we'd send
    // three requests for each connected cluster when the search bar gets open.
    if (filters.length >= 1) {
      return 'preview';
    }
    return 'no-search';
  }
  return 'full-search';
}

function getLengthScore(searchTerm: string, matchedValue: string): number {
  return Math.floor((searchTerm.length / matchedValue.length) * 100);
}
