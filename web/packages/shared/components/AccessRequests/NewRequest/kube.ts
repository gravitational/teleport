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

import { Attempt } from 'shared/hooks/useAttemptNext';

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
 * Checks each data for kube_cluster or namespace
 */
export function checkForUnsupportedKubeRequestModes(
  requestRoleAttempt: Attempt
) {
  let unsupportedKubeRequestModes = '';
  let affectedKubeClusterName = '';
  let requiresNamespaceSelect = false;

  if (requestRoleAttempt.status === 'failed') {
    const errMsg = requestRoleAttempt.statusText.toLowerCase();

    if (errMsg.includes('request_mode') && errMsg.includes('allowed kinds: ')) {
      const allowedKinds = errMsg.split('allowed kinds: ')[1];

      // Web UI supports selecting namespace and wildcard
      // which basically means requiring namespace.
      if (allowedKinds.includes('*') || allowedKinds.includes('namespace')) {
        requiresNamespaceSelect = true;
      } else {
        unsupportedKubeRequestModes = allowedKinds;
      }

      const initialSplit = errMsg.split('for kubernetes cluster');
      if (initialSplit.length > 1) {
        affectedKubeClusterName = initialSplit[1]
          .split('. allowed kinds')[0]
          .trim();
      }

      return {
        affectedKubeClusterName,
        requiresNamespaceSelect,
        unsupportedKubeRequestModes,
      };
    }
  }

  return {
    affectedKubeClusterName,
    unsupportedKubeRequestModes,
    requiresNamespaceSelect,
  };
}
