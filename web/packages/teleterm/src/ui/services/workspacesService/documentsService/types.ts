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

import { Report } from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import {
  ResourceHealthStatus,
  SharedUnifiedResource,
} from 'shared/components/UnifiedResources';

import type * as tsh from 'teleterm/services/tshd/types';
import * as uri from 'teleterm/ui/uri';

export type Kind = Document['kind'];

/**
 * DocumentOrigin denotes which part of Connect UI was used to create a document for the resource.
 */
export type DocumentOrigin =
  | 'resource_table'
  | 'search_bar'
  | 'connection_list'
  | 'reopened_session';

interface DocumentBase {
  uri: uri.DocumentUri;
  title: string;
  kind: Kind;
}

export interface DocumentBlank extends DocumentBase {
  kind: 'doc.blank';
}

export interface DocumentTshNode extends DocumentBase {
  kind: 'doc.terminal_tsh_node';
  // status is used merely to show a progress bar when the document is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
  rootClusterId: string;
  leafClusterId: string | undefined;
  origin: DocumentOrigin;
  // serverId is the UUID of the SSH server. If it's is present, we can immediately start an SSH
  // session.
  //
  // serverId is available when connecting to a server from the resource table.
  serverId: string;
  // serverUri is used for file transfer and for identifying a specific server among different
  // profiles and clusters.
  serverUri: uri.ServerUri;
  login: string;
}

/**
 * DocumentGateway is used for database and app gateways. The two are distinguished by the kind of
 * resource that targetUri points to.
 */
export interface DocumentGateway extends DocumentBase {
  kind: 'doc.gateway';
  /** status is used merely to show a progress bar when the gateway is being set up. */
  status: '' | 'connecting' | 'connected' | 'error';
  /**
   * gatewayUri is not present until the gateway described by the document is created.
   */
  gatewayUri?: uri.GatewayUri;
  targetUri: uri.DatabaseUri | uri.AppUri;
  /**
   * targetUser is used only for db gateways and must contain the db user. Connect allows only a
   * single doc.gateway to exist per targetUri + targetUser combo.
   */
  targetUser: string;
  /**
   * targetName contains the name of the target resource as shown in the UI. This field could be
   * removed in favor of targetUri, which always includes the target name anyway.
   */
  targetName: string;
  /**
   * targetSubresourceName contains database name for db gateways and target port for TCP app
   * gateways. A DocumentGateway created for a multi-port TCP app is expected to always have this
   * field present.
   *
   * For app gateways, Connect allows only a single doc.gateway to exist per targetUri +
   * targetSubresourceName combo.
   *
   * For db gateways, targetSubresourceName is not taken into account when considering document
   * "uniqueness".
   */
  targetSubresourceName: string | undefined;
  /**
   * port is the local port on which the gateway accepts connections.
   *
   * If empty, tshd is going to created a listener on a random port and then this field will be
   * updated to match that random port.
   */
  port?: string;
  origin: DocumentOrigin;
}

/**
 * DocumentGatewayCliClient is the tab that opens a CLI tool which targets the given gateway.
 *
 * The gateway is found by matching targetUri and targetUser rather than gatewayUri. gatewayUri
 * changes between app restarts while targetUri and targetUser won't.
 */
export interface DocumentGatewayCliClient extends DocumentBase {
  kind: 'doc.gateway_cli_client';
  // rootClusterId and leafClusterId are tech debt. They could be read from targetUri, but
  // useDocumentTerminal expects these fields to be set on the doc.
  rootClusterId: string;
  leafClusterId: string | undefined;
  // The four target properties are needed in order to call connectToDatabase from within
  // DocumentGatewayCliClient. targetName is needed to set a proper tab title.
  //
  // targetUri and targetUser are also needed to find a gateway providing the connection to the
  // target.
  targetUri: uri.DatabaseUri;
  targetUser: tsh.Gateway['targetUser'];
  targetName: tsh.Gateway['targetName'];
  targetProtocol: tsh.Gateway['protocol'];
  // status is used merely to show a progress bar when the doc waits for the gateway to be created.
  // It will be changed to 'connected' as soon as the CLI client prints something out. Some clients
  // type something out immediately after starting while others only after they actually connect to
  // a resource.
  status: '' | 'connecting' | 'connected' | 'error';
}

/**
 * DocumentGatewayKube transparently sets up a local proxy for the given kube cluster and spins up
 * a local shell session with KUBECONFIG pointing at the config managed by the local proxy.
 */
export interface DocumentGatewayKube extends DocumentBase {
  kind: 'doc.gateway_kube';
  rootClusterId: string;
  leafClusterId: string | undefined;
  targetUri: uri.KubeUri;
  origin: DocumentOrigin;
  /** Identifier of the shell to be opened. */
  shellId?: string;
  // status is used merely to show a progress bar when the gateway is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
}

export interface DocumentCluster extends DocumentBase {
  kind: 'doc.cluster';
  clusterUri: uri.ClusterUri;
  queryParams: DocumentClusterQueryParams;
}

// When extending this type, remember to update the
// `WorkspacesService.reopenPreviousDocuments` method
// that spreads all of its properties.
export interface DocumentClusterQueryParams {
  search: string;
  advancedSearchEnabled: boolean;
  /**
   * This is a list of 'resource kind' filters that can be selected from
   * both the search bar and the types selector in the unified resources view.
   *
   * If it is empty, all resource kinds are listed.
   */
  resourceKinds: DocumentClusterResourceKind[];
  sort: {
    fieldName: string;
    dir: 'ASC' | 'DESC';
  };
  statuses: ResourceHealthStatus[];
}

// Any changes done to this type must be backwards compatible as
// `DocumentClusterQueryParams` uses values of this type and documents are stored to disk.
export type DocumentClusterResourceKind = Extract<
  SharedUnifiedResource['resource']['kind'],
  'node' | 'app' | 'kube_cluster' | 'db' | 'windows_desktop'
>;

export interface DocumentAccessRequests extends DocumentBase {
  kind: 'doc.access_requests';
  clusterUri: uri.ClusterUri;
  state: AccessRequestDocumentState;
  requestId: string;
}

export interface DocumentPtySession extends DocumentBase {
  kind: 'doc.terminal_shell';
  cwd?: string;
  /** Identifier of the shell to be opened. */
  shellId?: string;
  rootClusterId?: string;
  leafClusterId?: string;
}

export interface DocumentConnectMyComputer extends DocumentBase {
  kind: 'doc.connect_my_computer';
  // `DocumentConnectMyComputer` always operates on the root cluster, so in theory `rootClusterUri` is not needed.
  // However, there are a few components in the system, such as `getResourceUri`, which need to determine the relation
  // between a document and a cluster just by looking at the document fields.
  rootClusterUri: uri.RootClusterUri;
  /**
   * The status of 'connecting' is used to indicate that Connect My Computer permissions cannot be
   * established yet and the document is waiting for the app to receive full cluster details.
   */
  status: '' | 'connecting' | 'connected' | 'error';
}

export interface DocumentVnetDiagReport extends DocumentBase {
  kind: 'doc.vnet_diag_report';
  // VNet itself is not bound to any workspace, but a document must belong to a workspace. Also, it
  // must be possible to determine the relation between a document and a cluster just by looking at
  // the document fields, hence why rootClusterUri is defined here.
  rootClusterUri: uri.RootClusterUri;
  report: Report;
}

export interface DocumentVnetInfo extends DocumentBase {
  kind: 'doc.vnet_info';
  // VNet itself is not bound to any workspace, but a document must belong to a workspace. Also, it
  // must be possible to determine the relation between a document and a cluster just by looking at
  // the document fields, hence why rootClusterUri is defined here.
  rootClusterUri: uri.RootClusterUri;
  /**
   * Details of the app if the doc was opened by selecting a specific TCP app.
   *
   * This field is needed to facilitate a scenario where a first-time user clicks "Connect" next to
   * a TCP app, which opens this doc. Once the user clicks "Start VNet" in the doc, Connect should
   * continue the regular flow of connecting to a TCP app through VNet, which means it should copy
   * the address of the app to the clipboard, hence this field.
   *
   * app is removed when restoring persisted state. Let's say the user opens the doc through the
   * "Connect" button of a specific app. If they close the app and then reopen the docs, we don't
   * want the "Start VNet" button to copy the address of the app from the prev session.
   */
  app:
    | {
        /**
         * The address that's going to be copied to the clipboard after user starts VNet for the
         * first time through this document.
         *
         */
        targetAddress: string | undefined;
        isMultiPort: boolean;
      }
    | undefined;
}

/**
 * Document to authorize a web session with device trust.
 * Unlike other documents, it is not persisted on disk.
 */
export interface DocumentAuthorizeWebSession extends DocumentBase {
  kind: 'doc.authorize_web_session';
  // `DocumentAuthorizeWebSession` always operates on the root cluster, so in theory `rootClusterUri` is not needed.
  // However, there are a few components in the system, such as `getResourceUri`, which need to determine the relation
  // between a document and a cluster just by looking at the document fields.
  rootClusterUri: uri.RootClusterUri;
  webSessionRequest: WebSessionRequest;
}

export interface DocumentDesktopSession extends DocumentBase {
  kind: 'doc.desktop_session';
  desktopUri: uri.DesktopUri;
  login: string;
  origin: DocumentOrigin;
  // status is used merely to indicate that a connection is established in the connection tracker.
  status: '' | 'connected' | 'error';
}

export interface WebSessionRequest {
  id: string;
  token: string;
  username: string;
  redirectUri: string;
}

export type DocumentTerminal =
  | DocumentPtySession
  | DocumentGatewayCliClient
  | DocumentTshNode
  | DocumentGatewayKube;

export type Document =
  | DocumentAccessRequests
  | DocumentBlank
  | DocumentGateway
  | DocumentCluster
  | DocumentTerminal
  | DocumentConnectMyComputer
  | DocumentVnetDiagReport
  | DocumentVnetInfo
  | DocumentAuthorizeWebSession
  | DocumentDesktopSession;

/**
 * `DocumentPtySession` and `DocumentGatewayKube` spawn a shell.
 * The shell is taken from the `doc.shellId` property.
 */
export function canDocChangeShell(
  doc: Document
): doc is DocumentPtySession | DocumentGatewayKube {
  return doc.kind === 'doc.terminal_shell' || doc.kind === 'doc.gateway_kube';
}

export type CreateGatewayDocumentOpts = {
  gatewayUri?: uri.GatewayUri;
  targetUri: uri.DatabaseUri | uri.AppUri;
  targetName: string;
  targetUser: string;
  targetSubresourceName?: string;
  title?: string;
  port?: string;
  origin: DocumentOrigin;
};

export type CreateAccessRequestDocumentOpts = {
  clusterUri: uri.ClusterUri;
  state: AccessRequestDocumentState;
  requestId?: string;
};

export type AccessRequestDocumentState = 'browsing' | 'creating' | 'reviewing';
