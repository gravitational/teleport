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

import { IAppContext } from 'teleterm/ui/types';
import { SearchResult } from 'teleterm/ui/Search/searchResult';
import { SearchContext } from 'teleterm/ui/Search/SearchContext';
import {
  connectToDatabase,
  connectToKube,
  connectToServer,
  connectToApp,
  DocumentCluster,
  getDefaultDocumentClusterQueryParams,
} from 'teleterm/ui/services/workspacesService';
import { retryWithRelogin, assertUnreachable } from 'teleterm/ui/utils';
import { routing } from 'teleterm/ui/uri';

export interface SimpleAction {
  type: 'simple-action';
  searchResult: SearchResult;
  preventAutoInputReset?: boolean;
  preventAutoClose?: boolean;
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

export function mapToAction(
  ctx: IAppContext,
  searchContext: SearchContext,
  result: SearchResult
): SearchAction {
  switch (result.kind) {
    case 'server': {
      return {
        type: 'parametrized-action',
        searchResult: result,
        parameter: {
          getSuggestions: async () =>
            ctx.clustersService.findClusterByResource(result.resource.uri)
              ?.loggedInUser?.sshLogins,
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
    case 'kube': {
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
    case 'app': {
      return {
        type: 'simple-action',
        searchResult: result,
        perform: () =>
          connectToApp(ctx, result.resource, {
            origin: 'search_bar',
          }),
      };
    }
    case 'database': {
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
    case 'resource-type-filter': {
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
    case 'cluster-filter': {
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
    case 'display-results': {
      return {
        type: 'simple-action',
        searchResult: result,
        preventAutoInputReset: true,
        perform: async () => {
          const rootClusterUri = routing.ensureRootClusterUri(
            result.clusterUri
          );
          if (result.documentUri) {
            ctx.workspacesService
              .getWorkspaceDocumentService(rootClusterUri)
              .update(result.documentUri, (draft: DocumentCluster) => {
                const { queryParams } = draft;
                queryParams.resourceKinds = result.resourceKinds;
                queryParams.search = result.value;
                queryParams.advancedSearchEnabled =
                  searchContext.advancedSearchEnabled;
              });
            return;
          }

          const { isAtDesiredWorkspace } =
            await ctx.workspacesService.setActiveWorkspace(rootClusterUri);
          if (isAtDesiredWorkspace) {
            const documentsService =
              ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
            const doc = documentsService.createClusterDocument({
              clusterUri: result.clusterUri,
              queryParams: {
                ...getDefaultDocumentClusterQueryParams(),
                search: result.value,
                advancedSearchEnabled: false,
                resourceKinds: result.resourceKinds,
              },
            });
            documentsService.add(doc);
            documentsService.open(doc.uri);
          }
        },
      };
    }
    default:
      assertUnreachable(result);
  }
}
