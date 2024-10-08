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

import { KubeResourceKind } from 'teleport/services/kube';

import { PendingListItem } from './RequestCheckout';

export type KubeNamespaceRequest = {
  kubeCluster: string;
  search: string;
};

/**
 * Returns true if the currItem is a kube cluster or is a namespace
 * of the currItem.
 *
 * Technically you can request for both a kube_cluster and its subresources
 * but it's probably not what a user would expect from the web UI because
 * requesting for subresources means narrowing access versus requesting
 * access to the whole kube_cluster.
 */
export function excludeKubeClusterWithNamespaces(
  currItem: PendingListItem,
  allItems: PendingListItem[]
) {
  return (
    currItem.kind !== 'kube_cluster' ||
    !(
      currItem.kind === 'kube_cluster' &&
      allItems.find(a => a.kind === 'namespace' && a.id == currItem.id)
    )
  );
}

/**
 * Returns flags on what kind of kubernetes requests a user
 * is allowed to make.
 */
export function getKubeResourceRequestMode(
  allowedKubeSubresourceKinds: KubeResourceKind[],
  hasSelectedKube
) {
  const canRequestKubeCluster = allowedKubeSubresourceKinds.length === 0;

  const canRequestKubeResource =
    allowedKubeSubresourceKinds.includes('*') || canRequestKubeCluster;

  const canRequestKubeNamespace =
    canRequestKubeResource || allowedKubeSubresourceKinds.includes('namespace');

  // This can happen if an admin restricts requests to a subresource
  // kind that the web UI does not support yet.
  const disableCheckoutFromKubeRestrictions =
    hasSelectedKube &&
    !canRequestKubeCluster &&
    !canRequestKubeResource &&
    !canRequestKubeNamespace;

  return {
    canRequestKubeCluster,
    canRequestKubeResource,
    canRequestKubeNamespace,
    disableCheckoutFromKubeRestrictions,
  };
}
