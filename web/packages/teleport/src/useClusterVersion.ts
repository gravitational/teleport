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

import { parse } from 'shared/utils/semVer';

import useTeleport from 'teleport/useTeleport';

/**
 * **useClusterVersion** returns the cluster (auth) version and a comparison
 * utility (`checkCompatibility`). The check utility can be used to compare the
 * providedclient version to the cluster/control-plane version. An indication
 * of cluster compatibility is returned.
 *
 * @returns the cluster (auth) version and a comparison utility (diff)
 */
export function useClusterVersion(): {
  clusterVersion: string;
  /**
   * **checkCompatibility** compares the provided client version to the cluster/
   * control-plane version. An indication of cluster compatibility is returned.
   * @param version the compare version as string.
   * @returns an indication of cluster compatibility
   */
  checkCompatibility: (
    clientVersion: string | undefined
  ) => ClientCompatibility | null;
} {
  const ctx = useTeleport();
  const clusterVersion = ctx.storeUser.getClusterAuthVersion();
  return {
    clusterVersion,
    checkCompatibility: (clientVersion?: string) =>
      checkClientCompatibility(clientVersion, clusterVersion),
  };
}

export type ClientCompatibility =
  | {
      isCompatible: false;
      reason: 'too-new' | 'too-old';
    }
  | {
      isCompatible: true;
      /**
       * match - versions are the same.
       * upgrade-major - the client is one version behind.
       * upgrade-minor - the major version is the same, but older on minor or
       * patch.
       */
      reason: 'match' | 'upgrade-major' | 'upgrade-minor';
    }
  | null;

export function checkClientCompatibility(
  clientVersion: string | undefined,
  clusterVersion: string
): ClientCompatibility {
  const client = parse(clientVersion);
  const cluster = parse(clusterVersion);
  if (!client || !cluster) return null;
  if (client.major === cluster.major) {
    return {
      isCompatible: true,
      reason: client.compare(cluster) === -1 ? 'upgrade-minor' : 'match',
    };
  }
  if (Math.abs(client.major - cluster.major) == 1) {
    return {
      isCompatible: true,
      reason: client.major > cluster.major ? 'match' : 'upgrade-major',
    };
  }
  return {
    isCompatible: false,
    reason: client.major > cluster.major ? 'too-new' : 'too-old',
  };
}
