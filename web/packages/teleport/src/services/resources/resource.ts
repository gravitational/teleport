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

import api from 'teleport/services/api';
import cfg, { UrlResourcesParams, UrlListRolesParams } from 'teleport/config';

import { UnifiedResource, ResourcesResponse } from '../agents';
import { KeysEnum } from '../storageService';

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

        // TODO (avatus) DELETE IN 15.0
        // if this request succeeds, we don't need a legacy view
        localStorage.removeItem(KeysEnum.UNIFIED_RESOURCES_NOT_SUPPORTED);
        return {
          agents: items.map(makeUnifiedResource),
          startKey: json?.startKey,
          totalCount: json?.totalCount,
        };
      })
      .catch(res => {
        // TODO (avatus) : a temporary check to catch unimplemented errors for unified resources
        // This is a quick hacky way to catch the error until we migrate completely to unified resources
        // DELETE IN 15.0
        if (
          (res.response?.status === 404 &&
            res.message?.includes('unknown method ListUnifiedResources')) ||
          res.response?.status === 501
        ) {
          localStorage.setItem(
            KeysEnum.UNIFIED_RESOURCES_NOT_SUPPORTED,
            'true'
          );
        }
        throw res;
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
