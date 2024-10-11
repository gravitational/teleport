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

import { ResourceLabel } from 'teleport/services/agents';
export interface Kube {
  kind: 'kube_cluster';
  name: string;
  labels: ResourceLabel[];
  users?: string[];
  groups?: string[];
  requiresRequest?: boolean;
}

/**
 * Add kind consts as we go.
 * Supported kube subresources:
 * https://github.com/gravitational/teleport/blob/c86f46db17fe149240e30fa0748621239e36c72a/api/types/constants.go#L1233
 */
export type KubeResourceKind = 'namespace';

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
