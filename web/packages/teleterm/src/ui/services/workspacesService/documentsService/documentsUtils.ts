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

import { ClusterOrResourceUri, routing } from 'teleterm/ui/uri';

import { Document, isDocumentTshNodeWithServerId } from './types';

/**
 * getResourceUri returns the URI of the cluster resource that is the subject of the document.
 *
 * For example, for DocumentGateway it's targetUri rather than gatewayUri because the gateway
 * doesn't belong to the cluster.
 *
 * At the moment it's used only to get the breadcrumbs for the status bar.
 */
export function getResourceUri(
  document: Document
): ClusterOrResourceUri | undefined {
  switch (document.kind) {
    case 'doc.cluster':
      return document.clusterUri;
    case 'doc.gateway':
    case 'doc.gateway_cli_client':
    case 'doc.gateway_kube':
      return document.targetUri;
    case 'doc.terminal_tsh_node':
      return isDocumentTshNodeWithServerId(document)
        ? document.serverUri
        : undefined;
    case 'doc.terminal_tsh_kube':
      return document.kubeUri;
    case 'doc.access_requests':
      return document.clusterUri;
    case 'doc.terminal_shell':
      return routing.getClusterUri({
        rootClusterId: document.rootClusterId,
        leafClusterId: document.leafClusterId,
      });
    case 'doc.connect_my_computer':
      return document.rootClusterUri;
    case 'doc.blank':
      return undefined;
    default:
      document satisfies never;
      return undefined;
  }
}
