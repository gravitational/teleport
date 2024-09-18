/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

/**
 * Returns a unique ID for a kube namespace
 * by appending namespace to the cluster it belongs to.
 *
 * Required since the same namespace can be present in
 * different kube clusters.
 */
export function getKubeNamespaceId({
  namespace,
  kubeCluster,
}: {
  namespace: string;
  kubeCluster: string;
}) {
  // using slash here is safe (to extract for later in
  // 'function extractKubeNamspaceFromId') as kubernetes
  // forbids slashes in namespaces.
  // https://github.com/gravitational/teleport/blob/80877f8b3dff098918e6b75e821a3bc7c05ceeeb/api/types/resource_ids.go#L100
  return `${kubeCluster}/${namespace}`;
}

export function extractKubeNamspaceFromId(id: string) {
  const values = id.split('/');
  if (values.length) {
    // The last in array will always be the namespace.
    // See 'function getKubeNamespaceId'
    return values[values.length - 1];
  }
  return '';
}

export type KubeNamespaceRequest = {
  kubeCluster: string;
  search: string;
};
