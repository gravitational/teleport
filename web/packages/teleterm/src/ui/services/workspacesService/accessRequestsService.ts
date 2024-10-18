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
  KubeResourceNamespaceUri,
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

  /**
   * Bulk action where if request is added, removes it or if request doesn't
   * exist, adds it.
   */
  async addOrRemoveResources(requestedResources: ResourceRequest[]) {
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

      requestedResources.forEach(request => {
        if (request.kind === 'namespace') {
          this.addOrRemoveKubeNamespace(request, resources);
          return;
        }
        if (resources.has(request.resource.uri)) {
          resources.delete(request.resource.uri);
        } else {
          resources.set(request.resource.uri, getRequiredProperties(request));
        }
      });
    });
  }

  async addOrRemoveKubeNamespace(
    namespaceResourceRequest: ResourceRequest,
    resources: Map<ResourceUri, ResourceRequest>
  ) {
    const { uri: resourceUri } = namespaceResourceRequest.resource;

    const requestedResource = resources.get(
      routing.getKubeUri(
        routing.parseKubeResourceNamespaceUri(resourceUri).params
      )
    );
    if (!requestedResource || requestedResource.kind !== 'kube') {
      throw new Error('Cannot add a kube namespace to a non-kube resource');
    }
    const kubeResource = requestedResource.resource;

    if (!kubeResource.namespaces) {
      kubeResource.namespaces = new Map();
    }
    if (kubeResource.namespaces.has(resourceUri)) {
      kubeResource.namespaces.delete(resourceUri);
    } else {
      kubeResource.namespaces.set(resourceUri, namespaceResourceRequest);
    }
  }

  /**
   * Removes all requested resources, if all the requested resources were already added
   * or adds all requested resources, if not all requested resources were added.
   *
   * Typically used when user "selects all or deselects all"
   */
  async addAllOrRemoveAllResources(requestedResources: ResourceRequest[]) {
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
      const allAdded = requestedResources.every(r =>
        resources.has(r.resource.uri)
      );

      requestedResources.forEach(request => {
        if (allAdded) {
          resources.delete(request.resource.uri);
        } else {
          resources.set(request.resource.uri, getRequiredProperties(request));
        }
      });
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
  if (kind === 'app') {
    return {
      kind,
      resource: { uri: resource.uri, samlApp: resource.samlApp },
    };
  }
  if (kind === 'namespace') {
    return {
      kind,
      resource: { uri: resource.uri },
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
        namespaces?: Map<ResourceUri, ResourceRequest>;
      };
    }
  | {
      kind: 'app';
      resource: {
        uri: AppUri;
        samlApp: boolean;
      };
    }
  | {
      kind: 'namespace';
      resource: {
        uri: KubeResourceNamespaceUri;
      };
    };

type SharedResourceAccessRequestKind =
  | 'app'
  | 'db'
  | 'node'
  | 'kube_cluster'
  | 'saml_idp_service_provider'
  | 'namespace';

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
   * Can refer to a pretty name of the resource (can be the same as `id`)
   * or refer to a subresource name.
   *
   * For example:
   * - for nodes, we want to show hostname (pretty) instead of its id.
   * - for a kube subresource like "namespace", it'll refer to its name
   *
   */
  name: string;
} {
  switch (kind) {
    case 'app': {
      const { appId } = routing.parseAppUri(resource.uri).params;
      if (resource.samlApp) {
        return { kind: 'saml_idp_service_provider', id: appId, name: appId };
      }
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
    case 'namespace': {
      const { kubeNamespaceId, kubeId } = routing.parseKubeResourceNamespaceUri(
        resource.uri
      ).params;
      return { kind, id: kubeId, name: kubeNamespaceId };
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
          samlApp: false,
        },
        kind: 'app',
      };
    case 'saml_idp_service_provider':
      return {
        resource: {
          uri: routing.getAppUri({
            rootClusterId,
            leafClusterId,
            appId: resourceId,
          }),
          samlApp: true,
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
    case 'namespace':
      return {
        resource: {
          uri: routing.getKubeResourceNamespaceUri({
            rootClusterId,
            leafClusterId,
            kubeId: resourceId,
            kubeNamespaceId: resourceName,
          }),
        },
        kind,
      };
    default:
      kind satisfies never;
  }
}
