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

import { ComponentType } from 'react';

import {
  Application,
  Broadcast,
  Database,
  Desktop,
  Kubernetes,
  Laptop,
  ListAddCheck,
  ListMagnifyingGlass,
  Server,
  ShieldCheck,
  Table,
  Terminal,
} from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';

import {
  ClusterOrResourceUri,
  isAppUri,
  isDatabaseUri,
  routing,
} from 'teleterm/ui/uri';

import { Document, DocumentGateway } from './types';

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
      return document.serverUri;
    case 'doc.access_requests':
      return document.clusterUri;
    case 'doc.terminal_shell':
      return routing.getClusterUri({
        rootClusterId: document.rootClusterId,
        leafClusterId: document.leafClusterId,
      });
    case 'doc.connect_my_computer':
      return document.rootClusterUri;
    case 'doc.authorize_web_session':
      return document.rootClusterUri;
    case 'doc.vnet_diag_report':
      return document.rootClusterUri;
    case 'doc.vnet_info':
      return document.rootClusterUri;
    case 'doc.desktop_session':
      return document.desktopUri;
    case 'doc.blank':
      return undefined;
    default:
      document satisfies never;
      return undefined;
  }
}

/**
 * getDocumentGatewayTargetUriKind is used when the callsite needs to distinguish between different
 * kinds of targets that DocumentGateway supports when given only its target URI.
 */
export function getDocumentGatewayTargetUriKind(
  targetUri: DocumentGateway['targetUri']
): 'db' | 'app' {
  if (isDatabaseUri(targetUri)) {
    return 'db';
  }

  if (isAppUri(targetUri)) {
    return 'app';
  }

  // TODO(ravicious): Optimally we'd use `targetUri satisfies never` here to have a type error when
  // DocumentGateway['targetUri'] is changed.
  //
  // However, at the moment that field is essentially of type string, so there's not much we can do
  // with regards to type safety.
}

export function getDocumentGatewayTitle(doc: DocumentGateway): string {
  const { targetName, targetUri, targetUser, targetSubresourceName } = doc;
  const targetKind = getDocumentGatewayTargetUriKind(targetUri);

  switch (targetKind) {
    case 'db': {
      return targetUser ? `${targetName} (${targetUser})` : targetName;
    }
    case 'app': {
      return targetSubresourceName
        ? `${targetName}:${targetSubresourceName}`
        : targetName;
    }
    default: {
      targetKind satisfies never;
    }
  }
}

/**
 * Returns a name and icon of the document.
 * If possible, the name is the title of a document, except for cases
 * when it contains some additional values like cwd, or a shell name.
 * At the moment, the name is used only in the status bar.
 * The icon is used both in the status bar and the tabs.
 */
export function getStaticNameAndIcon(
  document: Document
): { name: string; Icon: ComponentType<IconProps> } | undefined {
  switch (document.kind) {
    case 'doc.cluster':
      return {
        name: 'Resources',
        Icon: Table,
      };
    case 'doc.gateway_cli_client':
      return {
        name: document.title,
        Icon: Database,
      };
    case 'doc.gateway':
      if (isDatabaseUri(document.targetUri)) {
        return {
          name: document.title,
          Icon: Database,
        };
      }
      if (isAppUri(document.targetUri)) {
        return {
          name: document.title,
          Icon: Application,
        };
      }
      return;
    case 'doc.gateway_kube':
      return {
        name: routing.parseKubeUri(document.targetUri).params.kubeId,
        Icon: Kubernetes,
      };
    case 'doc.terminal_tsh_node':
      return {
        name: document.title,
        Icon: Server,
      };
    case 'doc.access_requests':
      return {
        name: document.title,
        Icon: ListAddCheck,
      };
    case 'doc.terminal_shell':
      return {
        name: 'Terminal',
        Icon: Terminal,
      };
    case 'doc.connect_my_computer':
      return {
        name: document.title,
        Icon: Laptop,
      };
    case 'doc.authorize_web_session':
      return {
        name: document.title,
        Icon: ShieldCheck,
      };
    case 'doc.vnet_diag_report':
      return {
        name: document.title,
        Icon: ListMagnifyingGlass,
      };
    case 'doc.vnet_info':
      return {
        name: document.title,
        Icon: Broadcast,
      };
    case 'doc.desktop_session':
      return {
        name: document.title,
        Icon: Desktop,
      };
    case 'doc.blank':
      return undefined;
    default:
      document satisfies never;
      return undefined;
  }
}
