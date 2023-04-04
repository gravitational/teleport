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
import { routing } from 'teleterm/ui/uri';
import { GatewayProtocol } from 'teleterm/services/tshd/types';
import { SearchResult } from 'teleterm/ui/Search/searchResult';
import { SearchContext } from 'teleterm/ui/Search/SearchContext';

export interface SimpleAction {
  type: 'simple-action';
  searchResult: SearchResult;
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
        perform(login) {
          ctx.commandLauncher.executeCommand('tsh-ssh', {
            localClusterUri: result.resource.uri,
            loginHost: `${login}@${result.resource.hostname}`,
            origin: 'search_bar',
          });
        },
      };
    }
    if (result.kind === 'kube') {
      return {
        type: 'simple-action',
        searchResult: result,
        perform() {
          ctx.commandLauncher.executeCommand('kube-connect', {
            kubeUri: result.resource.uri,
            origin: 'search_bar',
          });
        },
      };
    }
    if (result.kind === 'database') {
      return {
        type: 'parametrized-action',
        searchResult: result,
        parameter: {
          getSuggestions: () =>
            ctx.resourcesService.getDbUsers(result.resource.uri),
          placeholder: 'Provide db username',
        },
        async perform(dbUsername) {
          const rootClusterUri = routing.ensureRootClusterUri(
            result.resource.uri
          );
          const documentsService =
            ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);

          const doc = documentsService.createGatewayDocument({
            // Not passing the `gatewayUri` field here, as at this point the gateway doesn't exist yet.
            // `port` is not passed as well, we'll let the tsh daemon pick a random one.
            targetUri: result.resource.uri,
            targetName: result.resource.name,
            targetUser: getTargetUser(
              result.resource.protocol as GatewayProtocol,
              dbUsername
            ),
            origin: 'search_bar',
          });

          await ctx.workspacesService.setActiveWorkspace(rootClusterUri);

          const connectionToReuse =
            ctx.connectionTracker.findConnectionByDocument(doc);

          if (connectionToReuse) {
            ctx.connectionTracker.activateItem(connectionToReuse.id, {
              origin: 'search_bar',
            });
          } else {
            documentsService.add(doc);
            documentsService.open(doc.uri);
          }
        },
      };
    }
    if (result.kind === 'resource-type-filter') {
      return {
        type: 'simple-action',
        searchResult: result,
        preventAutoClose: true,
        perform() {
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
        perform() {
          searchContext.setFilter({
            filter: 'cluster',
            clusterUri: result.resource.uri,
          });
        },
      };
    }
  });
}

function getTargetUser(
  protocol: GatewayProtocol,
  providedDbUser: string
): string {
  // we are replicating tsh behavior (user can be omitted for Redis)
  // https://github.com/gravitational/teleport/blob/796e37bdbc1cb6e0a93b07115ffefa0e6922c529/tool/tsh/db.go#L240-L244
  // but unlike tsh, Connect has to provide a user that is then used in a gateway document
  if (protocol === 'redis') {
    return providedDbUser || 'default';
  }

  return providedDbUser;
}
