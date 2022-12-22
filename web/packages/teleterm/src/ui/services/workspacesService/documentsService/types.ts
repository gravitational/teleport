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

export interface DocumentTshNode extends DocumentBase {
  kind: 'doc.terminal_tsh_node';
  status: 'connecting' | 'connected' | 'disconnected';
  serverId: string;
  serverUri: uri.ServerUri;
  rootClusterId: string;
  leafClusterId?: string;
  login?: string;
}

export interface DocumentTshKube extends DocumentBase {
  kind: 'doc.terminal_tsh_kube';
  status: 'connecting' | 'connected' | 'disconnected';
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
