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

import * as shared from 'shared/services/types';

import * as tsh from 'teleterm/services/tshd/types';

export type SyncStatus = {
  status: 'processing' | 'ready' | 'failed' | '';
  statusText?: string;
};

export type KindTsh = 'tsh.cluster' | 'tsh.server' | 'tsh.app' | 'tsh.db';

export type PreferredMfaType = shared.PreferredMfaType;

export type Auth2faType = shared.Auth2faType;

export type AuthProviderType = shared.AuthProviderType;

export type AuthProvider = tsh.AuthProvider;

export type LoginParams = tsh.LoginParams;

export type Application = tsh.Application;

export type CreateGatewayParams = tsh.CreateGatewayParams;

export type GatewayProtocol = tsh.GatewayProtocol;

export type Gateway = tsh.Gateway;

export type Server = tsh.Server;

export type Kube = tsh.Kube;

export type Database = tsh.Database;

export interface AuthSettings extends tsh.AuthSettings {
  secondFactor: Auth2faType;
  preferredMfa: PreferredMfaType;
}

export { tsh };

export type ClustersServiceState = {
  clusters: Map<string, tsh.Cluster>;
  gateways: Map<string, tsh.Gateway>;
  apps: Map<string, tsh.Application>;
  servers: Map<string, tsh.Server>;
  kubes: Map<string, tsh.Kube>;
  dbs: Map<string, tsh.Database>;
  kubesSyncStatus: Map<string, SyncStatus>;
  appsSyncStatus: Map<string, SyncStatus>;
  serversSyncStatus: Map<string, SyncStatus>;
  dbsSyncStatus: Map<string, SyncStatus>;
};
