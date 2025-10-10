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

import { ResourceTargetHealth } from 'shared/components/UnifiedResources';

import { ResourceLabel } from 'teleport/services/agents';

export interface Kube {
  kind: 'kube_cluster';
  name: string;
  labels: ResourceLabel[];
  users?: string[];
  groups?: string[];
  requiresRequest?: boolean;
  /**
   * targetHealth describes the health status of a Kubernetes cluster
   * reported from an agent (kubernetes_service) that is proxying this
   * Kubernetes cluster.
   *
   * This field will be empty if the Kubernetes cluster was not extracted from
   * a kube_server resource. The following endpoints will set this field
   * since these endpoints query for kube_server under the hood and then
   * extract kube from it:
   * - webapi/sites/:site/resources (unified resources)
   */
  targetHealth?: ResourceTargetHealth;
}

/**
 * Only the web UI supported kinds are defined.
 * All supported backend kube subresources:
 * https://github.com/gravitational/teleport/blob/c86f46db17fe149240e30fa0748621239e36c72a/api/types/constants.go#L1233
 *
 * Wildcard means any of the kube subresources.
 */
export type KubeResourceKind = 'namespace' | '*';

/**
 * Refers to kube_cluster's subresources like namespaces, pods, etc
 */
export type KubeResource = {
  kind: KubeResourceKind;
  name: string;
  /**
   * namespace will be left blank, if the field `kind` is `namespace`
   */
  namespace?: string;
  labels: ResourceLabel[];
  /**
   * the kube cluster where this subresource belongs to
   */
  cluster: string;
};

export interface KubeResourceResponse {
  items: KubeResource[];
  startKey?: string;
  totalCount?: number;
}

export type KubeServer = {
  hostname: string;
  hostId: string;
  targetHealth?: ResourceTargetHealth;
};
