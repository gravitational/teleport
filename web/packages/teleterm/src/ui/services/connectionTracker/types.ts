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

import { AppUri, DatabaseUri, KubeUri, ServerUri } from 'teleterm/ui/uri';

type TrackedConnectionBase = {
  connected: boolean;
  id: string;
  title: string;
};

export interface TrackedServerConnection extends TrackedConnectionBase {
  kind: 'connection.server';
  title: string;
  serverUri: ServerUri;
  login: string;
}

export interface TrackedGatewayConnection extends TrackedConnectionBase {
  kind: 'connection.gateway';
  targetUri: DatabaseUri | AppUri;
  targetName: string;
  targetUser?: string;
  port?: string;
  targetSubresourceName?: string;
}

export interface TrackedKubeConnection extends TrackedConnectionBase {
  kind: 'connection.kube';
  /**
   * @deprecated Used only by connections created by doc.terminal_tsh_kube.
   */
  kubeConfigRelativePath?: string;
  kubeUri: KubeUri;
}

export type TrackedConnection =
  | TrackedServerConnection
  | TrackedGatewayConnection
  | TrackedKubeConnection;

export type ExtendedTrackedConnection = TrackedConnection & {
  clusterName: string;
};
