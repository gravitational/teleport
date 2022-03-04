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

export type Kind =
  | 'doc.cluster'
  | 'doc.blank'
  | 'doc.gateway'
  | 'doc.terminal_shell'
  | 'doc.terminal_tsh_node'
  | 'doc.terminal_tsh_kube';

interface DocumentBase {
  uri: string;
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
  serverUri: string;
  rootClusterId: string;
  leafClusterId?: string;
  login: string;
}

export interface DocumentTshKube extends DocumentBase {
  kind: 'doc.terminal_tsh_kube';
  status: 'connecting' | 'connected' | 'disconnected';
  kubeId: string;
  kubeUri: string;
  rootClusterId: string;
  leafClusterId?: string;
}

export interface DocumentGateway extends DocumentBase {
  kind: 'doc.gateway';
  gatewayUri: string;
  targetUri: string;
  targetUser?: string;
  port?: string;
}

export interface DocumentCluster extends DocumentBase {
  kind: 'doc.cluster';
  clusterUri: string;
}

export interface DocumentPtySession extends DocumentBase {
  kind: 'doc.terminal_shell';
  cwd?: string;
}

export type DocumentTerminal =
  | DocumentPtySession
  | DocumentTshNode
  | DocumentTshKube;

export type Document =
  | DocumentBlank
  | DocumentGateway
  | DocumentCluster
  | DocumentTerminal;

export type CreateGatewayDocumentOpts = {
  gatewayUri: string;
  targetUri: string;
  targetUser?: string;
  title: string;
  port?: string;
};
