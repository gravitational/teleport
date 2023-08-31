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
import cfg, { UrlResourcesParams } from 'teleport/config';
import history from 'teleport/services/history';

import { UnifiedResource, ResourcesResponse } from '../agents';
import { KeysEnum } from '../localStorage';

import { makeUnifiedResource } from './makeUnifiedResource';

import { makeResource, makeResourceList } from './';

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
        if (res.response?.status === 404 || res.response?.status === 501) {
          localStorage.setItem(
            KeysEnum.UNIFIED_RESOURCES_NOT_SUPPORTED,
            'true'
          );
          history.replace(cfg.getNodesRoute(clusterId));
        }
        throw res;
      });
  }

  fetchGithubConnectors() {
    return api
      .get(cfg.getGithubConnectorsUrl())
      .then(res => makeResourceList<'github'>(res));
  }

  fetchRoles() {
    return api
      .get(cfg.getRolesUrl())
      .then(res => makeResourceList<'role'>(res));
  }

  createTrustedCluster(content: string) {
    return api
      .post(cfg.getTrustedClustersUrl(), { content })
      .then(res => makeResource<'trusted_cluster'>(res));
  }

  createRole(content: string) {
    return api
      .post(cfg.getRolesUrl(), { content })
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
      .put(cfg.getRolesUrl(name), { content })
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
    return api.delete(cfg.getRolesUrl(name));
  }

  deleteGithubConnector(name: string) {
    return api.delete(cfg.getGithubConnectorsUrl(name));
  }
}

export default ResourceService;
