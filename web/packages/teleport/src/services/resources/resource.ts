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

import cfg, { UrlListRolesParams, UrlResourcesParams } from 'teleport/config';
import api from 'teleport/services/api';

import { ResourcesResponse, UnifiedResource } from '../agents';
import auth, { MfaChallengeScope } from '../auth/auth';
import {
  CreateOrOverwriteGitServer,
  DefaultAuthConnector,
  GitServer,
  makeResource,
  makeResourceList,
  Resource,
  RoleResource,
} from './';
import { makeUnifiedResource } from './makeUnifiedResource';

class ResourceService {
  createOrOverwriteGitServer(
    clusterId: string,
    req: CreateOrOverwriteGitServer
  ): Promise<GitServer> {
    return api.put(
      cfg.getGitServerUrl({ clusterId }, 'createOrOverwrite'),
      req
    );
  }

  deleteGitServer(clusterId: string, name: string): Promise<GitServer> {
    return api.delete(cfg.getGitServerUrl({ clusterId, name }, 'delete'));
  }

  getGitServer(clusterId: string, name: string): Promise<GitServer> {
    return api.get(cfg.getGitServerUrl({ clusterId, name }, 'get'));
  }

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

  async fetchGithubConnectors(): Promise<{
    defaultConnector: DefaultAuthConnector;
    connectors: Resource<'github'>[];
  }> {
    // MFA reuse needs to be allowed in case we need to fallback to another default connector
    const challengeResponse =
      await await auth.getMfaChallengeResponseForAdminAction(true);

    return api
      .get(cfg.getGithubConnectorsUrl(), undefined, challengeResponse)
      .then(res => ({
        defaultConnector: {
          name: res.defaultConnectorName,
          type: res.defaultConnectorType,
        },
        connectors: makeResourceList<'github'>(res.connectors),
      }));
  }

  async setDefaultAuthConnector(req: DefaultAuthConnector | { type: 'local' }) {
    // This is an admin action that needs an mfa challenge with reuse allowed.
    const challenge = await auth.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
      allowReuse: true,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    });

    const challengeResponse = await auth.getMfaChallengeResponse(challenge);

    return api.put(cfg.api.defaultConnectorPath, req, challengeResponse);
  }

  async fetchRoles(params?: UrlListRolesParams): Promise<{
    items: RoleResource[];
    startKey: string;
  }> {
    return await api.get(cfg.getListRolesUrl(params));
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

  fetchGithubConnector(name: string) {
    return api
      .get(cfg.getGithubConnectorUrl(name))
      .then(res => makeResource<'github'>(res));
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
