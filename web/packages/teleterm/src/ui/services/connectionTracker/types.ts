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

type TrackedConnectionBase = {
  connected: boolean;
  id: string;
  title: string;
};

export interface TrackedServerConnection extends TrackedConnectionBase {
  kind: 'connection.server';
  title: string;
  serverUri: string;
  login: string;
}

export interface TrackedGatewayConnection extends TrackedConnectionBase {
  kind: 'connection.gateway';
  targetUri: string;
  targetName: string;
  targetUser?: string;
  port?: string;
  gatewayUri: string;
  targetSubresourceName?: string;
}

export interface TrackedKubeConnection extends TrackedConnectionBase {
  kind: 'connection.kube';
  kubeConfigRelativePath: string;
  kubeUri: string;
}

export type TrackedConnection =
  | TrackedServerConnection
  | TrackedGatewayConnection
  | TrackedKubeConnection;

export type ExtendedTrackedConnection = TrackedConnection & {
  clusterName: string;
};
