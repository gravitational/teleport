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

import {
  ResourceUri,
  routing,
  ClusterUri,
  ServerUri,
  DatabaseUri,
  KubeUri,
  AppUri,
} from 'teleterm/ui/uri';
import { ModalsService } from 'teleterm/ui/services/modals';

export class AccessRequestsService {
  constructor(
    private modalsService: ModalsService,
    private getState: () => {
      isBarCollapsed: boolean;
      pending: PendingAccessRequest;
    },
    private setState: (
      draftState: (draft: {
        isBarCollapsed: boolean;
        pending: PendingAccessRequest;
      }) => void
    ) => void
  ) {}

  getCollapsed() {
    return this.getState().isBarCollapsed;
  }

  toggleBar() {
    this.setState(draftState => {
      draftState.isBarCollapsed = !draftState.isBarCollapsed;
    });
  }

  getPendingAccessRequest() {
    return this.getState().pending;
  }

  clearPendingAccessRequest() {
    this.setState(draftState => {
      draftState.pending = getEmptyPendingAccessRequest();
    });
  }

  getAddedItemsCount(): number {
    const pendingAccessRequest = this.getState().pending;
    const { kind } = pendingAccessRequest;
    switch (kind) {
      case 'role':
        return pendingAccessRequest.roles.size;
      case 'resource':
        return pendingAccessRequest.resources.size;
      default:
        kind satisfies never;
        return 0;
    }
  }

  async addOrRemoveResource(request: ResourceRequest): Promise<void> {
    if (!(await this.canUpdateRequest('resource'))) {
      return;
    }
    this.setState(draftState => {
      if (draftState.pending.kind !== 'resource') {
        draftState.pending = {
          kind: 'resource',
          resources: new Map(),
        };
      }

      const { resources } = draftState.pending;

      if (resources.has(request.resource.uri)) {
        resources.delete(request.resource.uri);
      } else {
        resources.set(request.resource.uri, getRequiredProperties(request));
      }
    });
  }

  async addResource(request: ResourceRequest): Promise<void> {
    if (!(await this.canUpdateRequest('resource'))) {
      return;
    }
    this.setState(draftState => {
      if (draftState.pending.kind !== 'resource') {
        draftState.pending = {
          kind: 'resource',
          resources: new Map(),
        };
      }

      const { resources } = draftState.pending;

      if (resources.has(request.resource.uri)) {
        return;
      }
      resources.set(request.resource.uri, getRequiredProperties(request));
    });
  }

  async addOrRemoveRole(role: string): Promise<void> {
    if (!(await this.canUpdateRequest('role'))) {
      return;
    }
    this.setState(draftState => {
      if (draftState.pending.kind !== 'role') {
        draftState.pending = {
          kind: 'role',
          roles: new Set(),
        };
      }

      const { roles } = draftState.pending;
      if (roles.has(role)) {
        roles.delete(role);
      } else {
        roles.add(role);
      }
    });
  }

  /**
   * Combining role access request and resource access request is not allowed.
   * If the user already has an item for one group, we need to ask
   * if they want to clear the request before adding items from another group.
   */
  private async canUpdateRequest(
    newRequestKind: 'resource' | 'role'
  ): Promise<boolean> {
    let shouldProceed = true;
    if (
      this.getState().pending.kind !== newRequestKind &&
      this.getAddedItemsCount() > 0
    ) {
      shouldProceed = await new Promise(resolve =>
        this.modalsService.openRegularDialog({
          kind: 'change-access-request-kind',
          onCancel: () => resolve(false),
          onConfirm: () => resolve(true),
        })
      );
    }
    return shouldProceed;
  }
}

/** Returns only the properties required by the type. */
function getRequiredProperties({
  kind,
  resource,
}: ResourceRequest): ResourceRequest {
  if (kind === 'server') {
    return {
      kind,
      resource: { uri: resource.uri, hostname: resource.hostname },
    };
  }
  return {
    kind,
    resource: { uri: resource.uri },
  };
}

/** Returns an empty access request. We default to the resource access request. */
export function getEmptyPendingAccessRequest(): PendingAccessRequest {
  return {
    kind: 'resource',
    resources: new Map(),
  };
}

export type PendingAccessRequest =
  | {
      kind: 'resource';
      resources: Map<ResourceUri, ResourceRequest>;
    }
  | { kind: 'role'; roles: Set<string> };

/**
 * Describes a resource in a resource access request.
 * This shape allows us to store certain properties for particular kinds,
 * e.g., hostname for a server.
 * Moreover, it matches the shape of a resource in the search bar
 * or in the unified resources view, making adding resources easier.
 *
 * In the future we can consider reusing this structure outside Connect,
 * but it would require replacing the uri with id and cluster name.
 */
export type ResourceRequest =
  | {
      kind: 'server';
      resource: {
        uri: ServerUri;
        hostname: string;
      };
    }
  | {
      kind: 'database';
      resource: {
        uri: DatabaseUri;
      };
    }
  | {
      kind: 'kube';
      resource: {
        uri: KubeUri;
      };
    }
  | {
      kind: 'app';
      resource: {
        uri: AppUri;
      };
    }
  | {
      kind: 'saml_idp_service_provider';
      resource: {
        uri: AppUri;
      };
    };

type SharedResourceAccessRequestKind =
  | 'app'
  | 'db'
  | 'node'
  | 'kube_cluster'
  | 'saml_idp_service_provider';

/**
 * Extracts `kind`, `id` and `name` from the resource request.
 * The extracted kind uses *shared resource kinds*
 * since this function is used only to provide values for the shared code.
 */
export function extractResourceRequestProperties({
  kind,
  resource,
}: ResourceRequest): {
  kind: SharedResourceAccessRequestKind;
  id: string;
  /**
   * Pretty name of the resource (can be the same as `id`).
   * For example, for nodes, we want to show hostname instead of its id.
   */
  name: string;
} {
  switch (kind) {
    case 'app': {
      const { appId } = routing.parseAppUri(resource.uri).params;
      return { kind: 'app', id: appId, name: appId };
    }
    case 'server': {
      const { serverId } = routing.parseServerUri(resource.uri).params;
      return { kind: 'node', id: serverId, name: resource.hostname };
    }
    case 'database': {
      const { dbId } = routing.parseDbUri(resource.uri).params;
      return { kind: 'db', id: dbId, name: dbId };
    }
    case 'kube': {
      const { kubeId } = routing.parseKubeUri(resource.uri).params;
      return { kind: 'kube_cluster', id: kubeId, name: kubeId };
    }
    case 'saml_idp_service_provider': {
      const { appId } = routing.parseAppUri(resource.uri).params;
      return { kind: 'saml_idp_service_provider', id: appId, name: appId };
    }
    default:
      kind satisfies never;
  }
}

/**
 * Maps the type used by the shared access requests to the type
 * required by the access requests service.
 */
export function toResourceRequest({
  kind,
  clusterUri,
  resourceId,
  resourceName,
}: {
  kind: SharedResourceAccessRequestKind;
  clusterUri: ClusterUri;
  resourceId: string;
  resourceName?: string;
}): ResourceRequest {
  const {
    params: { rootClusterId, leafClusterId },
  } = routing.parseClusterUri(clusterUri);

  switch (kind) {
    case 'app':
      return {
        resource: {
          uri: routing.getAppUri({
            rootClusterId,
            leafClusterId,
            appId: resourceId,
          }),
        },
        kind: 'app',
      };
    case 'db':
      return {
        resource: {
          uri: routing.getDbUri({
            rootClusterId,
            leafClusterId,
            dbId: resourceId,
          }),
        },
        kind: 'database',
      };
    case 'node':
      return {
        resource: {
          uri: routing.getServerUri({
            rootClusterId,
            leafClusterId,
            serverId: resourceId,
          }),
          hostname: resourceName,
        },
        kind: 'server',
      };
    case 'kube_cluster':
      return {
        resource: {
          uri: routing.getKubeUri({
            rootClusterId,
            leafClusterId,
            kubeId: resourceId,
          }),
        },
        kind: 'kube',
      };
    case 'saml_idp_service_provider':
      return {
        resource: {
          uri: routing.getAppUri({
            rootClusterId,
            leafClusterId,
            appId: resourceId,
          }),
        },
        kind: 'saml_idp_service_provider',
      };
    default:
      kind satisfies never;
  }
}
