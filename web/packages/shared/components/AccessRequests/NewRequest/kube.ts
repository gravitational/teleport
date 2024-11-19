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

/**
 * Returns true if the item is a kube cluster or is a namespace
 * of the item.
 *
 * Technically you can request for both a kube_cluster and its subresources
 * but it's probably not what a user would expect from the web UI because
 * requesting for subresources means narrowing access versus requesting
 * access to the whole kube_cluster.
 */
export function isKubeClusterWithNamespaces(
  item: PendingListItem,
  allItems: PendingListItem[]
) {
  return (
    item.kind === 'kube_cluster' &&
    allItems.find(a => a.kind === 'namespace' && a.id == item.id)
  );
}

/**
 * Parses error message (in an expected format returned from the backend) to see if
 * the message is a type of request.kubernetes_resources related error:
 * https://github.com/gravitational/teleport/blob/master/lib/services/access_request.go#L2050
 *
 * If error matches, it then checks if namespace or kube_cluster is mentioned
 * in the "allowed kinds" portion of the error message.
 */
export function checkSupportForKubeResources(requestRoleAttempt: Attempt) {
  let requestKubeResourceSupported = true;
  let isRequestKubeResourceError = false;

  const retVal = { requestKubeResourceSupported, isRequestKubeResourceError };
  if (requestRoleAttempt.status === 'failed') {
    const errMsg = requestRoleAttempt.statusText.toLowerCase();

    if (
      errMsg.includes('request.kubernetes_resources') &&
      errMsg.includes('allowed kinds')
    ) {
      let splitMsgParts = [];
      if (errMsg.includes('requested roles: ')) {
        splitMsgParts = errMsg.split('requested roles: ');
      } else if (errMsg.includes('requestable roles: ')) {
        splitMsgParts = errMsg.split('requestable roles: ');
      }

      if (!splitMsgParts.length || !splitMsgParts[1]) {
        return retVal;
      }

      const kindParts = splitMsgParts[1].split(', ');

      // Check that at least one of the kind parts have a kind that
      // the web UI supports (namespace or kube_cluster):
      const isSupported = kindParts.some(part => {
        const allowed = part.split(': ');
        if (
          allowed[1]?.includes('namespace') ||
          allowed[1]?.includes('kube_cluster')
        ) {
          return true;
        }
      });
      return {
        requestKubeResourceSupported: isSupported,
        isRequestKubeResourceError: true,
      };
    }
  }

  return retVal;
}
