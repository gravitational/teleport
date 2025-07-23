/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { UnreachableCluster } from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';
import { Cluster as TshdCluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import {
  AppUpdateEvent,
  AutoUpdatesStatus,
} from 'teleterm/services/appUpdater';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

import iconWinLinux from '../../../build_resources/icon-linux/512x512.png';
import iconMac from '../../../build_resources/icon-mac.png';

export { iconMac, iconWinLinux };

export function formatMB(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(2)} MB`;
}

export function findUnreachableClusters(
  status: AutoUpdatesStatus
): UnreachableCluster[] {
  if (status.enabled === true) {
    switch (status.source.kind) {
      case 'env-var':
        return [];
      case 'managing-cluster':
      case 'most-compatible':
        return status.source.unreachableClusters;
    }
  }

  switch (status.reason) {
    case 'disabled-by-env-var':
      return [];
    case 'no-compatible-version':
    case 'no-cluster-with-auto-update':
      return status.unreachableClusters;
  }
}

export function getDownloadHost(event: AppUpdateEvent): string {
  switch (event.kind) {
    case 'update-available':
    case 'download-progress':
    case 'update-downloaded':
      return new URL(event.update.files.at(0).url).host;
    default:
      return '';
  }
}

export function isTeleportDownloadHost(host: string): boolean {
  return ['cdn.teleport.dev', 'cdn.cloud.gravitational.io'].includes(host);
}

export interface ClusterGetter {
  findCluster(clusterUri: RootClusterUri): TshdCluster | undefined;
}

export const clusterNameGetter =
  (clusterService: ClusterGetter) => (clusterUri: RootClusterUri) =>
    clusterService.findCluster(clusterUri)?.name ||
    routing.parseClusterName(clusterUri);
