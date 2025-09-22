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

import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';
import { WindowsDesktop } from 'gen-proto-ts/teleport/lib/teleterm/v1/windows_desktop_pb';

import {
  resourceOneOfIsApp,
  resourceOneOfIsDatabase,
  resourceOneOfIsKube,
  resourceOneOfIsServer,
  resourceOneOfIsWindowsDesktop,
} from 'teleterm/helpers';
import Logger from 'teleterm/logger';
import type { TshdClient } from 'teleterm/services/tshd';
import { getAppAddrWithProtocol } from 'teleterm/services/tshd/app';
import {
  cloneAbortSignal,
  TshdRpcError,
} from 'teleterm/services/tshd/cloneableClient';
import type * as types from 'teleterm/services/tshd/types';
import { getWindowsDesktopAddrWithoutDefaultPort } from 'teleterm/services/tshd/windowsDesktop';
import type { ResourceTypeFilter } from 'teleterm/ui/Search/searchResult';
import type * as uri from 'teleterm/ui/uri';

export class ResourcesService {
  private logger = new Logger('ResourcesService');

  constructor(private tshClient: TshdClient) {}

  async getDbUsers(dbUri: uri.DatabaseUri): Promise<string[]> {
    const { response } = await this.tshClient.listDatabaseUsers({ dbUri });
    return response.users;
  }

  /**
   * searchResources searches for the given list of space-separated keywords across all resource
   * types on the given cluster.
   *
   * It does so by issuing a separate request for each resource type. It fails if any of those
   * requests fail.
   *
   * The results need to be wrapped in SearchResult because if we returned raw types (Server,
   * Database, Kube) then there would be no easy way to differentiate between them on type level.
   */
  async searchResources({
    clusterUri,
    search,
    filters,
    limit,
    includeRequestable,
  }: {
    clusterUri: uri.ClusterUri;
    search: string;
    filters: ResourceTypeFilter[];
    limit: number;
    includeRequestable: boolean;
  }): Promise<SearchResult[]> {
    try {
      const { resources } = await this.listUnifiedResources({
        clusterUri,
        kinds: filters,
        limit,
        search,
        query: '',
        searchAsRoles: false,
        pinnedOnly: false,
        startKey: '',
        sortBy: { field: 'name', isDesc: true },
        includeRequestable,
      });
      return resources.map(r => {
        if (r.kind === 'app') {
          return {
            ...r,
            resource: {
              ...r.resource,
              addrWithProtocol: getAppAddrWithProtocol(r.resource),
            },
          };
        }
        if (r.kind === 'windows_desktop') {
          return {
            ...r,
            resource: {
              ...r.resource,
              addrWithoutDefaultPort: getWindowsDesktopAddrWithoutDefaultPort(
                r.resource
              ),
            },
          };
        }
        return r;
      });
    } catch (err) {
      throw new ResourceSearchError(clusterUri, err);
    }
  }

  async listUnifiedResources(
    params: types.ListUnifiedResourcesRequest,
    abortSignal?: AbortSignal
  ): Promise<{ nextKey: string; resources: UnifiedResourceResponse[] }> {
    const { response } = await this.tshClient.listUnifiedResources(params, {
      abort: abortSignal && cloneAbortSignal(abortSignal),
    });
    return {
      nextKey: response.nextKey,
      resources: response.resources
        .map(p => {
          if (resourceOneOfIsServer(p.resource)) {
            return {
              kind: 'server' as const,
              resource: p.resource.server,
              requiresRequest: p.requiresRequest,
            };
          }

          if (resourceOneOfIsDatabase(p.resource)) {
            return {
              kind: 'database' as const,
              resource: p.resource.database,
              requiresRequest: p.requiresRequest,
            };
          }

          if (resourceOneOfIsApp(p.resource)) {
            return {
              kind: 'app' as const,
              resource: p.resource.app,
              requiresRequest: p.requiresRequest,
            };
          }

          if (resourceOneOfIsKube(p.resource)) {
            return {
              kind: 'kube' as const,
              resource: p.resource.kube,
              requiresRequest: p.requiresRequest,
            };
          }

          if (resourceOneOfIsWindowsDesktop(p.resource)) {
            return {
              kind: 'windows_desktop' as const,
              resource: p.resource.windowsDesktop,
              requiresRequest: p.requiresRequest,
            };
          }

          this.logger.info(
            `Ignoring unsupported resource ${JSON.stringify(p)}.`
          );
        })
        .filter(Boolean),
    };
  }
}

export class ResourceSearchError extends Error {
  constructor(
    public clusterUri: uri.ClusterUri,
    cause: Error | TshdRpcError
  ) {
    super(`Error while fetching resources from cluster ${clusterUri}`, {
      cause,
    });
    this.name = 'ResourceSearchError';
    this.clusterUri = clusterUri;
  }

  messageWithClusterName(
    getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string,
    opts = { capitalize: true }
  ) {
    const cluster = getClusterName(this.clusterUri);

    return `${
      opts.capitalize ? 'Could' : 'could'
    } not fetch resources from ${cluster}`;
  }

  messageAndCauseWithClusterName(
    getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string
  ) {
    return `${this.messageWithClusterName(getClusterName)}:\n${
      this.cause['message']
    }`;
  }
}

export type SearchResultServer = {
  kind: 'server';
  resource: types.Server;
  requiresRequest: boolean;
};
export type SearchResultDatabase = {
  kind: 'database';
  resource: types.Database;
  requiresRequest: boolean;
};
export type SearchResultKube = {
  kind: 'kube';
  resource: types.Kube;
  requiresRequest: boolean;
};
export type SearchResultApp = {
  kind: 'app';
  resource: App & { addrWithProtocol: string };
  requiresRequest: boolean;
};
export type SearchResultWindowsDesktop = {
  kind: 'windows_desktop';
  resource: WindowsDesktop & { addrWithoutDefaultPort: string };
  requiresRequest: boolean;
};

export type SearchResult =
  | SearchResultServer
  | SearchResultDatabase
  | SearchResultKube
  | SearchResultApp
  | SearchResultWindowsDesktop;

export type SearchResultResource<Kind extends SearchResult['kind']> =
  Kind extends 'server'
    ? SearchResultServer['resource']
    : Kind extends 'app'
      ? SearchResultApp['resource']
      : Kind extends 'database'
        ? SearchResultDatabase['resource']
        : Kind extends 'kube'
          ? SearchResultKube['resource']
          : Kind extends 'windows_desktop'
            ? SearchResultWindowsDesktop['resource']
            : never;

export type UnifiedResourceResponse =
  | { kind: 'server'; resource: types.Server; requiresRequest: boolean }
  | {
      kind: 'database';
      resource: types.Database;
      requiresRequest: boolean;
    }
  | { kind: 'kube'; resource: types.Kube; requiresRequest: boolean }
  | { kind: 'app'; resource: App; requiresRequest: boolean }
  | {
      kind: 'windows_desktop';
      resource: WindowsDesktop;
      requiresRequest: boolean;
    };
