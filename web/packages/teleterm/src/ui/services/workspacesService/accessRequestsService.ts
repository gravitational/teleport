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

export class AccessRequestsService {
  constructor(
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

  getAddedResourceCount(): number {
    const pendingAccessRequest = this.getState().pending;
    if (pendingAccessRequest.kind === 'role') {
      return pendingAccessRequest.roles.size;
    }
    if (pendingAccessRequest.kind === 'resource') {
      return pendingAccessRequest.resources.size;
    }

    return 0;
  }

  async addOrRemoveResource({
    kind,
    resource,
  }: ResourceRequest): Promise<void> {
    this.setState(draftState => {
      if (draftState.pending.kind !== 'resource') {
        draftState.pending = {
          kind: 'resource',
          resources: new Map(),
        };
      }

      const { resources } = draftState.pending;

      if (resources.has(resource.uri)) {
        resources.delete(resource.uri);
      } else {
        // Store only properties required by the type.
        if (kind === 'app' || kind === 'database' || kind === 'kube') {
          resources.set(resource.uri, {
            kind,
            resource: { uri: resource.uri },
          });
        } else {
          resources.set(resource.uri, {
            kind,
            resource: { uri: resource.uri, hostname: resource.hostname },
          });
        }
      }
    });
  }

  async addOrRemoveRole(role: string): Promise<void> {
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
    };

type SharedResourceAccessRequestKind = 'app' | 'db' | 'node' | 'kube_cluster';

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
    default:
      kind satisfies never;
  }
}
