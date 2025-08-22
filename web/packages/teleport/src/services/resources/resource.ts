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

import { AuthProvider } from 'shared/services';

import cfg, { UrlListRolesParams, UrlResourcesParams } from 'teleport/config';
import api from 'teleport/services/api';

import { ResourcesResponse, UnifiedResource } from '../agents';
import auth, { MfaChallengeScope } from '../auth/auth';
import { MfaChallengeResponse } from '../mfa';
import { isPathNotFoundError } from '../version/unsupported';
import { yamlService } from '../yaml';
import { YamlSupportedResourceKind } from '../yaml/types';
import {
  CreateOrOverwriteGitServer,
  DefaultAuthConnector,
  GitServer,
  makeResource,
  makeResourceList,
  RequestableRole,
  Resource,
  Role,
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

  async setDefaultAuthConnector(
    req: DefaultAuthConnector | { type: 'local' },
    abortSignal?: AbortSignal
  ) {
    // This is an admin action that needs an mfa challenge with reuse allowed.
    const challenge = await auth.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
      allowReuse: true,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    });

    const challengeResponse = await auth.getMfaChallengeResponse(challenge);

    return api.put(
      cfg.api.defaultConnectorPath,
      req,
      abortSignal,
      challengeResponse
    );
  }

  async getUserMatchedAuthConnectors(
    username: string
  ): Promise<AuthProvider[]> {
    return api
      .post(cfg.api.authConnectorsPath, { username })
      .then(res => res.connectors || []);
  }

  async fetchRoles(
    params?: UrlListRolesParams,
    signal?: AbortSignal
  ): Promise<{
    items: RoleResource[];
    startKey: string;
  }> {
    return await api.get(cfg.getRoleUrl({ action: 'list', params }), signal);
  }

  async fetchRequestableRoles(
    params: UrlListRolesParams,
    // TODO(rudream): DELETE IN v21.0. This is here now to maintain backwards compatibility with an older auth version
    // that may not have the requestable roles endpoint.
    allRequestableRoles: string[]
  ): Promise<{
    items: RequestableRole[];
    startKey: string;
  }> {
    return await api.get(cfg.getListRequestableRolesUrl(params)).catch(err => {
      // If the endpoint isn't found due to it being in an older version than the client, fallback to using the full list of requestable roles
      // to maintain compatibility with the paginated table component which expects a paginated response.
      // TODO(rudream): DELETE IN v21.0
      if (isPathNotFoundError(err)) {
        return makeRequestableRolesPageLocally(params, allRequestableRoles);
      } else {
        throw err;
      }
    });
  }

  fetchPresetRoles() {
    return api
      .get(cfg.getPresetRolesUrl())
      .then(res => makeResourceList<'role'>(res));
  }

  /**
   * @deprecated use standalone fetchRole function defined below this class
   */
  async fetchRole(name: string): Promise<RoleResource> {
    return makeResource<'role'>(
      await api.get(
        cfg.getRoleUrl({ action: 'get', name }),
        undefined,
        undefined
      )
    );
  }

  createTrustedCluster(content: string) {
    return api
      .post(cfg.getTrustedClustersUrl(), { content })
      .then(res => makeResource<'trusted_cluster'>(res));
  }

  createRole(content: string, mfaResponse?: MfaChallengeResponse) {
    return api
      .post(
        cfg.api.role.create,
        { content },
        undefined /* abort signal */,
        mfaResponse
      )
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

  /**
   * @deprecated use standalone updateRole function defined below this class
   */
  updateRole(name: string, content: string) {
    return api
      .put(cfg.getRoleUrl({ action: 'update', name }), { content })
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
    return api.delete(cfg.getRoleUrl({ action: 'delete', name }));
  }

  deleteGithubConnector(name: string) {
    return api.delete(cfg.getGithubConnectorsUrl(name));
  }
}

export default ResourceService;

export async function fetchRole(
  name: string,
  signal?: AbortSignal
): Promise<RoleResource> {
  return makeResource<'role'>(
    await api.get(cfg.getRoleUrl({ action: 'get', name }), signal, undefined)
  );
}

export async function fetchRoleWithYamlParse(name: string): Promise<Role> {
  const { content } = await fetchRole(name);
  return yamlService.parse<Role>(YamlSupportedResourceKind.Role, {
    yaml: content,
  });
}

export async function updateRoleWithYamlConversion({
  name,
  role,
}: {
  name: string;
  role: Role;
}) {
  const content = await yamlService.stringify(YamlSupportedResourceKind.Role, {
    resource: role,
  });
  return updateRole({ name, content });
}

export async function updateRole({
  name,
  content,
}: {
  name: string;
  content: string;
}): Promise<RoleResource> {
  return api
    .put(cfg.getRoleUrl({ action: 'update', name }), { content })
    .then(res => makeResource<'role'>(res));
}

/**
 * makeRequestableRolesPageLocally mocks a paginated response for requestable roles so that a list of all requestable roles
 * can be handled by the serverside paginated table component.
 */
// TODO(rudream): DELETE IN v21.0
function makeRequestableRolesPageLocally(
  params: UrlListRolesParams,
  allRequestableRoles: string[]
): {
  items: RequestableRole[];
  startKey: string;
} {
  if (params.search) {
    allRequestableRoles = allRequestableRoles.filter(r =>
      r.toLowerCase().includes(params.search.toLowerCase())
    );
  }

  if (params.startKey) {
    const startIndex = allRequestableRoles.findIndex(
      r => r === params.startKey
    );
    allRequestableRoles = allRequestableRoles.slice(startIndex);
  }

  const limit = params.limit || 200;
  const nextKey = allRequestableRoles.at(limit) || '';
  allRequestableRoles = allRequestableRoles.slice(0, limit);

  return {
    items: allRequestableRoles.map(r => ({ name: r, description: '' })),
    startKey: nextKey,
  };
}
