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
import { pluralize } from 'shared/utils/text';

import { AppUpdateEvent } from 'teleterm/services/appUpdater';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

import iconWinLinux from '../../../build_resources/icon-linux/512x512.png';
import iconMac from '../../../build_resources/icon-mac.png';

export { iconMac, iconWinLinux };

export function formatMB(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(2)} MB`;
}

export function getDownloadHost(event: AppUpdateEvent): string {
  switch (event.kind) {
    case 'update-available':
    case 'download-progress':
    case 'update-downloaded':
    case 'error':
      const url = event.update?.files?.at(0)?.url;
      return url && new URL(url).host;
    default:
      return '';
  }
}

export function isTeleportDownloadHost(host: string): boolean {
  return host === 'cdn.teleport.dev';
}

export interface ClusterGetter {
  findCluster(clusterUri: RootClusterUri): TshdCluster | undefined;
}

export const clusterNameGetter =
  (clusterService: ClusterGetter) => (clusterUri: RootClusterUri) =>
    clusterService.findCluster(clusterUri)?.name ||
    routing.parseClusterName(clusterUri);

const listFormatter = new Intl.ListFormat('en', {
  style: 'long',
  type: 'conjunction',
});

export function makeUnreachableClusterText(
  unreachableClusters: UnreachableCluster[],
  getClusterName: (clusterUri: RootClusterUri) => string
) {
  return (
    `Unable to retrieve accepted client versions` +
    ` from the ${pluralize(unreachableClusters.length, 'cluster')}` +
    ` ${listFormatter.format(unreachableClusters.map(c => getClusterName(c.clusterUri)))}.`
  );
}
