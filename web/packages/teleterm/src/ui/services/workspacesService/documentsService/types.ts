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

import { SharedUnifiedResource } from 'shared/components/UnifiedResources';

import * as uri from 'teleterm/ui/uri';

import type * as tsh from 'teleterm/services/tshd/types';

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

export type DocumentTshNode =
  | DocumentTshNodeWithServerId
  // DELETE IN 14.0.0
  //
  // Logging in to an arbitrary host was removed in 13.0 together with the command bar.
  // However, there's a slight chance that some users upgrading from 12.x to 13.0 still have
  // documents with loginHost in the app state (e.g. if the doc failed to connect to the server).
  // Let's just remove this in 14.0.0 instead to make sure those users can safely upgrade the app.
  | DocumentTshNodeWithLoginHost;

interface DocumentTshNodeBase extends DocumentBase {
  kind: 'doc.terminal_tsh_node';
  // status is used merely to show a progress bar when the document is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
  rootClusterId: string;
  leafClusterId: string | undefined;
  origin: DocumentOrigin;
}

export interface DocumentTshNodeWithServerId extends DocumentTshNodeBase {
  // serverId is the UUID of the SSH server. If it's is present, we can immediately start an SSH
  // session.
  //
  // serverId is available when connecting to a server from the resource table.
  serverId: string;
  // serverUri is used for file transfer and for identifying a specific server among different
  // profiles and clusters.
  serverUri: uri.ServerUri;
  // login is missing when the user executes `tsh ssh host` from the command bar without supplying
  // the login. In that case, login will be undefined and serverId will be equal to "host". tsh will
  // assume that login equals to the current OS user.
  login?: string;
  // loginHost exists on DocumentTshNodeWithServerId mostly because
  // DocumentsService.prototype.update doesn't let us remove fields. To keep the types truthful to
  // the implementation (which is something we should avoid doing, it should work the other way
  // around), loginHost was kept on DocumentTshNodeWithServerId.
  loginHost?: undefined;
}

export interface DocumentTshNodeWithLoginHost extends DocumentTshNodeBase {
  // serverId is missing, so we need to resolve loginHost to a server UUID.
  loginHost: string;
  // We don't provide types for other fields on purpose (such as serverId?: undefined) in order to
  // force places which use DocumentTshNode to narrow down the type before using it.
}

// DELETE IN 15.0.0. See DocumentGatewayKube for more details.
export interface DocumentTshKube extends DocumentBase {
  kind: 'doc.terminal_tsh_kube';
  // status is used merely to show a progress bar when the document is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
  kubeId: string;
  kubeUri: uri.KubeUri;
  kubeConfigRelativePath: string;
  rootClusterId: string;
  leafClusterId?: string;
  origin: DocumentOrigin;
}

export interface DocumentGateway extends DocumentBase {
  kind: 'doc.gateway';
  // status is used merely to show a progress bar when the gateway is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
  gatewayUri?: uri.GatewayUri;
  targetUri: uri.DatabaseUri | uri.AppUri;
  targetUser: string;
  targetName: string;
  targetSubresourceName?: string;
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
 * DocumentGatewayKube replaced DocumentTshKube in Connect v14. Before removing DocumentTshKube
 * completely, we should add a migration that transforms all DocumentTshKube docs into
 * DocumentGatewayKube docs when loading the workspace state from disk.
 */
export interface DocumentGatewayKube extends DocumentBase {
  kind: 'doc.gateway_kube';
  rootClusterId: string;
  leafClusterId: string | undefined;
  targetUri: uri.KubeUri;
  origin: DocumentOrigin;
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
}

// Any changes done to this type must be backwards compatible as
// `DocumentClusterQueryParams` uses values of this type and documents are stored to disk.
export type DocumentClusterResourceKind = Extract<
  SharedUnifiedResource['resource']['kind'],
  'node' | 'app' | 'kube_cluster' | 'db'
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

export type DocumentTerminal =
  | DocumentPtySession
  | DocumentGatewayCliClient
  | DocumentTshNode
  | DocumentTshKube
  | DocumentGatewayKube;

export type Document =
  | DocumentAccessRequests
  | DocumentBlank
  | DocumentGateway
  | DocumentCluster
  | DocumentTerminal
  | DocumentConnectMyComputer;

export function isDocumentTshNodeWithLoginHost(
  doc: Document
): doc is DocumentTshNodeWithLoginHost {
  // Careful here as TypeScript lets you make type guards unsound. You can double invert the last
  // check and TypeScript won't complain.
  return doc.kind === 'doc.terminal_tsh_node' && !('serverId' in doc);
}

export function isDocumentTshNodeWithServerId(
  doc: Document
): doc is DocumentTshNodeWithServerId {
  // Careful here as TypeScript lets you make type guards unsound. You can double invert the last
  // check and TypeScript won't complain.
  return doc.kind === 'doc.terminal_tsh_node' && 'serverId' in doc;
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

export type CreateTshKubeDocumentOptions = {
  kubeUri: uri.KubeUri;
  kubeConfigRelativePath?: string;
  origin: DocumentOrigin;
};

export type CreateAccessRequestDocumentOpts = {
  clusterUri: uri.ClusterUri;
  state: AccessRequestDocumentState;
  title?: string;
  requestId?: string;
};

export type AccessRequestDocumentState = 'browsing' | 'creating' | 'reviewing';
