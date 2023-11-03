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

import { IAppContext } from 'teleterm/ui/types';
import {
  SearchResult,
  ClusterSearchFilter,
  ResourceTypeSearchFilter,
} from 'teleterm/ui/Search/searchResult';
import { SearchContext } from 'teleterm/ui/Search/SearchContext';
import {
  connectToDatabase,
  connectToKube,
  connectToServer,
} from 'teleterm/ui/services/workspacesService';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { routing } from 'teleterm/ui/uri';

export interface SimpleAction {
  type: 'simple-action';
  searchResult: SearchResult;
  preventAutoClear?: boolean; // TODO(gzdunek): consider other options (callback preventClose() in perform?)
  preventAutoClose?: boolean; // TODO(gzdunek): consider other options (callback preventClose() in perform?)

  perform(): void;
}

export interface ParametrizedAction {
  type: 'parametrized-action';
  searchResult: SearchResult;
  preventAutoClose?: boolean;
  parameter: {
    getSuggestions(): Promise<string[]>;
    placeholder: string;
  };

  perform(parameter: string): void;
}

export type SearchAction = SimpleAction | ParametrizedAction;

export function mapToActions(
  ctx: IAppContext,
  searchContext: SearchContext,
  searchResults: SearchResult[]
): SearchAction[] {
  return searchResults.map(result => {
    if (result.kind === 'server') {
      return {
        type: 'parametrized-action',
        searchResult: result,
        parameter: {
          getSuggestions: async () =>
            ctx.clustersService.findClusterByResource(result.resource.uri)
              ?.loggedInUser?.sshLoginsList,
          placeholder: 'Provide login',
        },
        perform: login => {
          const { uri, hostname } = result.resource;
          return connectToServer(
            ctx,
            { uri, hostname, login },
            {
              origin: 'search_bar',
            }
          );
        },
      };
    }
    if (result.kind === 'kube') {
      return {
        type: 'simple-action',
        searchResult: result,
        perform: () => {
          const { uri } = result.resource;
          return connectToKube(
            ctx,
            { uri },
            {
              origin: 'search_bar',
            }
          );
        },
      };
    }
    if (result.kind === 'database') {
      return {
        type: 'parametrized-action',
        searchResult: result,
        parameter: {
          getSuggestions: () =>
            retryWithRelogin(ctx, result.resource.uri, () =>
              ctx.resourcesService.getDbUsers(result.resource.uri)
            ),
          placeholder: 'Provide db username',
        },
        perform: dbUser => {
          const { uri, name, protocol } = result.resource;
          return connectToDatabase(
            ctx,
            {
              uri,
              name,
              protocol,
              dbUser,
            },
            {
              origin: 'search_bar',
            }
          );
        },
      };
    }
    if (result.kind === 'resource-type-filter') {
      return {
        type: 'simple-action',
        searchResult: result,
        preventAutoClose: true,
        perform: () => {
          searchContext.setFilter({
            filter: 'resource-type',
            resourceType: result.resource,
          });
        },
      };
    }
    if (result.kind === 'cluster-filter') {
      return {
        type: 'simple-action',
        searchResult: result,
        preventAutoClose: true,
        perform: () => {
          searchContext.setFilter({
            filter: 'cluster',
            clusterUri: result.resource.uri,
          });
        },
      };
    }
    if (result.kind === 'unified-resource-filter') {
      return {
        type: 'simple-action',
        searchResult: result,
        preventAutoClear: true,
        perform: async () => {
          console.warn(searchContext.filters);
          const f = searchContext.filters.find(
            f => f.filter === 'cluster'
          ) as ClusterSearchFilter;
          const r = searchContext.filters.find(
            f => f.filter === 'resource-type'
          ) as ResourceTypeSearchFilter;
          if (!f) {
            if (
              ctx.resourceSearchService.getConnector()?.clusterUri ===
              ctx.workspacesService.getActiveWorkspace().localClusterUri
            ) {
              ctx.resourceSearchService.getConnector().setState(d => {
                d.search = result.value;
                d.kinds = mapResourceFilter(r);
                d.isAdvancedSearchEnabled = searchContext.advancedSearchEnabled;
              });
            } else {
              const { isAtDesiredWorkspace } =
                await ctx.workspacesService.setActiveWorkspace(
                  routing.ensureRootClusterUri(
                    ctx.workspacesService.getActiveWorkspace().localClusterUri
                  )
                );
              if (isAtDesiredWorkspace) {
                const docService =
                  ctx.workspacesService.getWorkspaceDocumentService(
                    routing.ensureRootClusterUri(
                      ctx.workspacesService.getActiveWorkspace().localClusterUri
                    )
                  );
                const doc = docService.createClusterDocument({
                  clusterUri:
                    ctx.workspacesService.getActiveWorkspace().localClusterUri,
                  initialQueryParams: {
                    search: result.value,
                    isAdvancedSearchEnabled: false,
                    kinds: mapResourceFilter(r),
                  },
                });
                docService.add(doc);
                docService.open(doc.uri);
              }
              return;
            }
            return;
          }

          if (
            ctx.resourceSearchService.getConnector()?.clusterUri ===
            f.clusterUri
          ) {
            ctx.resourceSearchService.getConnector().setState(d => {
              d.search = result.value;
              d.kinds = mapResourceFilter(r);
              d.isAdvancedSearchEnabled = searchContext.advancedSearchEnabled;
            });
          } else {
            const { isAtDesiredWorkspace } =
              await ctx.workspacesService.setActiveWorkspace(
                routing.ensureRootClusterUri(f.clusterUri)
              );
            if (isAtDesiredWorkspace) {
              const docService =
                ctx.workspacesService.getWorkspaceDocumentService(
                  routing.ensureRootClusterUri(f.clusterUri)
                );
              const doc = docService.createClusterDocument({
                clusterUri: f.clusterUri,
                initialQueryParams: {
                  search: result.value,
                  isAdvancedSearchEnabled: false,
                  kinds: mapResourceFilter(r),
                },
              });
              docService.add(doc);
              docService.open(doc.uri);
            }
            return;
          }
        },
      };
    }
  });
}

function mapResourceFilter(r: ResourceTypeSearchFilter) {
  if (r) {
    let kind;
    if (r.resourceType === 'servers') {
      kind = 'node';
    }
    if (r.resourceType === 'kubes') {
      kind = 'kube_cluster';
    }
    if (r.resourceType === 'databases') {
      kind = 'db';
    }
    return [kind];
  } else {
    return [];
  }
}
