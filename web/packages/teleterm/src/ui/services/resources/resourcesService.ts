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

import { pluralize } from 'shared/utils/text';

import type { ResourceTypeSearchFilter } from 'teleterm/ui/Search/searchResult';
import type * as types from 'teleterm/services/tshd/types';
import type * as uri from 'teleterm/ui/uri';

export class ResourcesService {
  constructor(private tshClient: types.TshClient) {}

  fetchServers(params: types.GetResourcesParams) {
    return this.tshClient.getServers(params);
  }

  // TODO(ravicious): Refactor it to use logic similar to that in the Web UI.
  // https://github.com/gravitational/teleport/blob/2a2b08dbfdaf71706a5af3812d3a7ec843d099b4/lib/web/apiserver.go#L2471
  async getServerByHostname(
    clusterUri: uri.ClusterUri,
    hostname: string
  ): Promise<types.Server | undefined> {
    const query = `name == "${hostname}"`;
    const { agentsList: servers } = await this.fetchServers({
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

  fetchDatabases(params: types.GetResourcesParams) {
    return this.tshClient.getDatabases(params);
  }

  fetchKubes(params: types.GetResourcesParams) {
    return this.tshClient.getKubes(params);
  }

  async getDbUsers(dbUri: uri.DatabaseUri): Promise<string[]> {
    return await this.tshClient.listDatabaseUsers(dbUri);
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
    filter,
    limit,
  }: {
    clusterUri: uri.ClusterUri;
    search: string;
    // TODO(ravicious): Accept just `server | database | kube` as searchFilter here, wrap it in a
    // variant of a discriminated union in searchResult.ts.
    filter: ResourceTypeSearchFilter | undefined;
    limit: number;
  }): Promise<PromiseSettledResult<SearchResult[]>[]> {
    const params = { search, clusterUri, sort: null, limit };

    const getServers = () =>
      this.fetchServers(params).then(
        res =>
          res.agentsList.map(resource => ({
            kind: 'server' as const,
            resource,
          })),
        err =>
          Promise.reject(new ResourceSearchError(clusterUri, 'server', err))
      );
    const getDatabases = () =>
      this.fetchDatabases(params).then(
        res =>
          res.agentsList.map(resource => ({
            kind: 'database' as const,
            resource,
          })),
        err =>
          Promise.reject(new ResourceSearchError(clusterUri, 'database', err))
      );
    const getKubes = () =>
      this.fetchKubes(params).then(
        res =>
          res.agentsList.map(resource => ({
            kind: 'kube' as const,
            resource,
          })),
        err => Promise.reject(new ResourceSearchError(clusterUri, 'kube', err))
      );

    const promises = filter
      ? [
          filter.resourceType === 'servers' && getServers(),
          filter.resourceType === 'databases' && getDatabases(),
          filter.resourceType === 'kubes' && getKubes(),
        ].filter(Boolean)
      : [getServers(), getDatabases(), getKubes()];

    return Promise.allSettled(promises);
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
    cause: Error
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

export type SearchResult =
  | SearchResultServer
  | SearchResultDatabase
  | SearchResultKube;

export type SearchResultResource<Kind extends SearchResult['kind']> =
  Kind extends 'server'
    ? SearchResultServer['resource']
    : Kind extends 'database'
    ? SearchResultDatabase['resource']
    : Kind extends 'kube'
    ? SearchResultKube['resource']
    : never;
