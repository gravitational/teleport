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

import { pluralize } from 'shared/utils/text';

import { makeApp, App } from 'teleterm/ui/services/clusters';

import {
  cloneAbortSignal,
  TshdRpcError,
} from 'teleterm/services/tshd/cloneableClient';

import {
  resourceOneOfIsServer,
  resourceOneOfIsDatabase,
  resourceOneOfIsApp,
  resourceOneOfIsKube,
} from 'teleterm/helpers';

import Logger from 'teleterm/logger';

import type { TshdClient } from 'teleterm/services/tshd';
import type * as types from 'teleterm/services/tshd/types';
import type * as uri from 'teleterm/ui/uri';
import type { ResourceTypeFilter } from 'teleterm/ui/Search/searchResult';

export class ResourcesService {
  private logger = new Logger('ResourcesService');

  constructor(private tshClient: TshdClient) {}

  async fetchServers(params: types.GetResourcesParams) {
    const { response } = await this.tshClient.getServers(
      makeGetResourcesParamsRequest(params)
    );
    return response;
  }

  // TODO(ravicious): Refactor it to use logic similar to that in the Web UI.
  // https://github.com/gravitational/teleport/blob/2a2b08dbfdaf71706a5af3812d3a7ec843d099b4/lib/web/apiserver.go#L2471
  async getServerByHostname(
    clusterUri: uri.ClusterUri,
    hostname: string
  ): Promise<types.Server | undefined> {
    const query = `name == "${hostname}"`;
    const { agents: servers } = await this.fetchServers({
      clusterUri,
      query,
      limit: 2,
      sort: null,
    });

    if (servers.length > 1) {
      throw new AmbiguousHostnameError(hostname);
    }

    return servers[0];
  }

  async fetchDatabases(params: types.GetResourcesParams) {
    const { response } = await this.tshClient.getDatabases(
      makeGetResourcesParamsRequest(params)
    );
    return response;
  }

  async fetchKubes(params: types.GetResourcesParams) {
    const { response } = await this.tshClient.getKubes(
      makeGetResourcesParamsRequest(params)
    );
    return response;
  }

  async fetchApps(params: types.GetResourcesParams) {
    const { response } = await this.tshClient.getApps(
      makeGetResourcesParamsRequest(params)
    );
    return response;
  }

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
  }: {
    clusterUri: uri.ClusterUri;
    search: string;
    filters: ResourceTypeFilter[];
    limit: number;
  }): Promise<PromiseSettledResult<SearchResult[]>[]> {
    const params = { search, clusterUri, sort: null, limit, startKey: '' };

    const getServers = () =>
      this.fetchServers(params).then(
        res =>
          res.agents.map(resource => ({
            kind: 'server' as const,
            resource,
          })),
        err =>
          Promise.reject(new ResourceSearchError(clusterUri, 'server', err))
      );
    const getApps = () =>
      this.fetchApps(params).then(
        res =>
          res.agents.map(resource => ({
            kind: 'app' as const,
            resource: makeApp(resource),
          })),
        err => Promise.reject(new ResourceSearchError(clusterUri, 'app', err))
      );
    const getDatabases = () =>
      this.fetchDatabases(params).then(
        res =>
          res.agents.map(resource => ({
            kind: 'database' as const,
            resource,
          })),
        err =>
          Promise.reject(new ResourceSearchError(clusterUri, 'database', err))
      );
    const getKubes = () =>
      this.fetchKubes(params).then(
        res =>
          res.agents.map(resource => ({
            kind: 'kube' as const,
            resource,
          })),
        err => Promise.reject(new ResourceSearchError(clusterUri, 'kube', err))
      );

    const promises = filters?.length
      ? [
          filters.includes('node') && getServers(),
          filters.includes('app') && getApps(),
          filters.includes('db') && getDatabases(),
          filters.includes('kube_cluster') && getKubes(),
        ].filter(Boolean)
      : [getServers(), getApps(), getDatabases(), getKubes()];

    return Promise.allSettled(promises);
  }

  async listUnifiedResources(
    params: types.ListUnifiedResourcesRequest,
    abortSignal: AbortSignal
  ): Promise<{ nextKey: string; resources: UnifiedResourceResponse[] }> {
    const { response } = await this.tshClient.listUnifiedResources(params, {
      abort: cloneAbortSignal(abortSignal),
    });
    return {
      nextKey: response.nextKey,
      resources: response.resources
        .map(p => {
          if (resourceOneOfIsServer(p.resource)) {
            return {
              kind: 'server' as const,
              resource: p.resource.server,
            };
          }

          if (resourceOneOfIsDatabase(p.resource)) {
            return {
              kind: 'database' as const,
              resource: p.resource.database,
            };
          }

          if (resourceOneOfIsApp(p.resource)) {
            return {
              kind: 'app' as const,
              resource: p.resource.app,
            };
          }

          if (resourceOneOfIsKube(p.resource)) {
            return {
              kind: 'kube' as const,
              resource: p.resource.kube,
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

export class AmbiguousHostnameError extends Error {
  constructor(hostname: string) {
    super(`Ambiguous hostname "${hostname}"`);
    this.name = 'AmbiguousHostname';
  }
}

export class ResourceSearchError extends Error {
  constructor(
    public clusterUri: uri.ClusterUri,
    public resourceKind: SearchResult['kind'],
    cause: Error | TshdRpcError
  ) {
    super(
      `Error while fetching resources of type ${resourceKind} from cluster ${clusterUri}`,
      { cause }
    );
    this.name = 'ResourceSearchError';
    this.clusterUri = clusterUri;
    this.resourceKind = resourceKind;
  }

  messageWithClusterName(
    getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string,
    opts = { capitalize: true }
  ) {
    const resource = pluralize(2, this.resourceKind);
    const cluster = getClusterName(this.clusterUri);

    return `${
      opts.capitalize ? 'Could' : 'could'
    } not fetch ${resource} from ${cluster}`;
  }

  messageAndCauseWithClusterName(
    getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string
  ) {
    return `${this.messageWithClusterName(getClusterName)}:\n${
      this.cause['message']
    }`;
  }
}

export type SearchResultServer = { kind: 'server'; resource: types.Server };
export type SearchResultDatabase = {
  kind: 'database';
  resource: types.Database;
};
export type SearchResultKube = { kind: 'kube'; resource: types.Kube };
export type SearchResultApp = {
  kind: 'app';
  resource: App;
};

export type SearchResult =
  | SearchResultServer
  | SearchResultDatabase
  | SearchResultKube
  | SearchResultApp;

export type SearchResultResource<Kind extends SearchResult['kind']> =
  Kind extends 'server'
    ? SearchResultServer['resource']
    : Kind extends 'app'
      ? SearchResultApp['resource']
      : Kind extends 'database'
        ? SearchResultDatabase['resource']
        : Kind extends 'kube'
          ? SearchResultKube['resource']
          : never;

function makeGetResourcesParamsRequest(params: types.GetResourcesParams) {
  return {
    ...params,
    search: params.search || '',
    query: params.query || '',
    searchAsRoles: params.searchAsRoles || '',
    startKey: params.startKey || '',
    sortBy: params.sort
      ? `${params.sort.fieldName}:${params.sort.dir.toLowerCase()}`
      : '',
  };
}

export type UnifiedResourceResponse =
  | { kind: 'server'; resource: types.Server }
  | {
      kind: 'database';
      resource: types.Database;
    }
  | { kind: 'kube'; resource: types.Kube }
  | { kind: 'app'; resource: types.App };
