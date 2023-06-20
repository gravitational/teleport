/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ClusterOrResourceUri, routing } from 'teleterm/ui/uri';
import { assertUnreachable } from 'teleterm/ui/utils';

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
    case 'doc.connect_my_computer_setup':
      return document.clusterUri;
    case 'doc.blank':
      return undefined;
    default:
      assertUnreachable(document);
  }
}
