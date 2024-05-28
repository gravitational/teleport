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

import api from 'teleport/services/api';
import cfg, { UrlResourcesParams, UrlListRolesParams } from 'teleport/config';

import { UnifiedResource, ResourcesResponse } from '../agents';

import { makeUnifiedResource } from './makeUnifiedResource';

import { makeResource, makeResourceList, RoleResource } from './';

class ResourceService {
  fetchTrustedClusters() {
    return api
      .get(cfg.getTrustedClustersUrl())
      .then(res => makeResourceList<'trusted_cluster'>(res));
  }

  fetchUnifiedResources(
    clusterId?: string,
    params?: UrlResourcesParams,
    signal?: AbortSignal
  ): Promise<ResourcesResponse<UnifiedResource>> {
    return api
      .get(cfg.getUnifiedResourcesUrl(clusterId, params), signal)
      .then(json => {
        const items = json?.items || [];

        return {
          agents: items.map(makeUnifiedResource),
          startKey: json?.startKey,
          totalCount: json?.totalCount,
        };
      });
  }

  fetchGithubConnectors() {
    return api
      .get(cfg.getGithubConnectorsUrl())
      .then(res => makeResourceList<'github'>(res));
  }

  async fetchRoles(params?: UrlListRolesParams): Promise<{
    items: RoleResource[];
    startKey: string;
  }> {
    const response = await api.get(cfg.getListRolesUrl(params));

    // This will handle backward compatibility with roles.
    // The old roles API returns only an array of resources while
    // the new one sends the paginated object with startKey/requests
    // If this webclient requests an older proxy
    // (this may happen in multi proxy deployments),
    //  this should allow the old request to not break the Web UI.
    // TODO (gzdunek): DELETE in 17.0.0
    if (Array.isArray(response)) {
      return makeRolesPageLocally(params, response);
    }

    return response;
  }

  fetchPresetRoles() {
    return api
      .get(cfg.getPresetRolesUrl())
      .then(res => makeResourceList<'role'>(res));
  }

  createTrustedCluster(content: string) {
    return api
      .post(cfg.getTrustedClustersUrl(), { content })
      .then(res => makeResource<'trusted_cluster'>(res));
  }

  createRole(content: string) {
    return api
      .post(cfg.getRoleUrl(), { content })
      .then(res => makeResource<'role'>(res));
  }

  createGithubConnector(content: string) {
    return api
      .post(cfg.getGithubConnectorsUrl(), { content })
      .then(res => makeResource<'github'>(res));
  }

  updateTrustedCluster(name: string, content: string) {
    return api
      .put(cfg.getTrustedClustersUrl(name), { content })
      .then(res => makeResource<'trusted_cluster'>(res));
  }

  updateRole(name: string, content: string) {
    return api
      .put(cfg.getRoleUrl(name), { content })
      .then(res => makeResource<'role'>(res));
  }

  updateGithubConnector(name: string, content: string) {
    return api
      .put(cfg.getGithubConnectorsUrl(name), { content })
      .then(res => makeResource<'github'>(res));
  }

  deleteTrustedCluster(name: string) {
    return api.delete(cfg.getTrustedClustersUrl(name));
  }

  deleteRole(name: string) {
    return api.delete(cfg.getRoleUrl(name));
  }

  deleteGithubConnector(name: string) {
    return api.delete(cfg.getGithubConnectorsUrl(name));
  }
}

export default ResourceService;

// TODO (gzdunek): DELETE in 17.0.0.
// See the comment where this function is used.
function makeRolesPageLocally(
  params: UrlListRolesParams,
  response: RoleResource[]
): {
  items: RoleResource[];
  startKey: string;
} {
  if (params.search) {
    // A serverside search would also match labels, here we only check the name.
    response = response.filter(p =>
      p.name.toLowerCase().includes(params.search.toLowerCase())
    );
  }

  if (params.startKey) {
    const startIndex = response.findIndex(p => p.name === params.startKey);
    response = response.slice(startIndex);
  }

  const limit = params.limit || 200;
  const nextKey = response.at(limit)?.name;
  response = response.slice(0, limit);

  return { items: response, startKey: nextKey };
}
