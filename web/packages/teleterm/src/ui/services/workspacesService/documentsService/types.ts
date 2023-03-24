/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import * as uri from 'teleterm/ui/uri';

export type Kind =
  | 'doc.access_requests'
  | 'doc.cluster'
  | 'doc.blank'
  | 'doc.gateway'
  | 'doc.terminal_shell'
  | 'doc.terminal_tsh_node'
  | 'doc.terminal_tsh_kube';

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
  | DocumentTshNodeWithLoginHost;

interface DocumentTshNodeBase extends DocumentBase {
  kind: 'doc.terminal_tsh_node';
  // status is used merely to show a progress bar when the document is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
  rootClusterId: string;
  leafClusterId: string | undefined;
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

export interface DocumentTshKube extends DocumentBase {
  kind: 'doc.terminal_tsh_kube';
  // status is used merely to show a progress bar when the document is being set up.
  status: '' | 'connecting' | 'connected' | 'error';
  kubeId: string;
  kubeUri: uri.KubeUri;
  kubeConfigRelativePath: string;
  rootClusterId: string;
  leafClusterId?: string;
}

export interface DocumentGateway extends DocumentBase {
  kind: 'doc.gateway';
  gatewayUri?: uri.GatewayUri;
  targetUri: uri.DatabaseUri;
  targetUser: string;
  targetName: string;
  targetSubresourceName?: string;
  port?: string;
}

export interface DocumentCluster extends DocumentBase {
  kind: 'doc.cluster';
  clusterUri: uri.ClusterUri;
}

export interface DocumentAccessRequests extends DocumentBase {
  kind: 'doc.access_requests';
  clusterUri: uri.ClusterUri;
  state: AccessRequestDocumentState;
  requestId: string;
}

export interface DocumentPtySession extends DocumentBase {
  kind: 'doc.terminal_shell';
  cwd?: string;
  initCommand?: string;
  rootClusterId?: string;
  leafClusterId?: string;
}

export type DocumentTerminal =
  | DocumentPtySession
  | DocumentTshNode
  | DocumentTshKube;

export type Document =
  | DocumentAccessRequests
  | DocumentBlank
  | DocumentGateway
  | DocumentCluster
  | DocumentTerminal;

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
  targetUri: uri.DatabaseUri;
  targetName: string;
  targetUser: string;
  targetSubresourceName?: string;
  title?: string;
  port?: string;
};

export type CreateClusterDocumentOpts = {
  clusterUri: uri.ClusterUri;
};

export type CreateTshKubeDocumentOptions = {
  kubeUri: uri.KubeUri;
  kubeConfigRelativePath?: string;
};

export type CreateAccessRequestDocumentOpts = {
  clusterUri: uri.ClusterUri;
  state: AccessRequestDocumentState;
  title?: string;
  requestId?: string;
};

export type AccessRequestDocumentState = 'browsing' | 'creating' | 'reviewing';

export type CreateNewTerminalOpts = {
  initCommand?: string;
  rootClusterId: string;
  leafClusterId?: string;
};
